package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type calibrateResult struct {
	Seconds float64 `json:"seconds"`
}

// MatchConfig contains configuration for a match
type MatchConfig struct {
	GridWidth   int           `json:"grid_width"`
	GridHeight  int           `json:"grid_height"`
	MaxTurns    int           `json:"max_turns"`
	TurnTimeout time.Duration `json:"turn_timeout"`
	// Optional scaling factor applied to TurnTimeout (e.g., from calibration)
	TurnTimeoutScale float64 `json:"turn_timeout_scale"`
	Bot1Dir          string  `json:"bot1_directory"`
	Bot2Dir          string  `json:"bot2_directory"`
	ReplayOutput     string  `json:"replay_output"`
	Verbose          bool    `json:"verbose"`
	MapPath          string  `json:"map_path"`
}

// TurnRecord records what happened in a single turn
type TurnRecord struct {
	Turn       int           `json:"turn"`
	GameState  *GameState    `json:"game_state"`
	Move1      Direction     `json:"move1"`
	Move2      Direction     `json:"move2"`
	Timeout1   bool          `json:"timeout1"`
	Timeout2   bool          `json:"timeout2"`
	TimeTaken1 time.Duration `json:"time_taken1"`
	TimeTaken2 time.Duration `json:"time_taken2"`
}

// MatchReplay contains the complete history of a match
type MatchReplay struct {
	Config      MatchConfig            `json:"config"`
	Turns       []TurnRecord           `json:"turns"`
	Winner      int                    `json:"winner"`
	WinReason   string                 `json:"win_reason"`
	TotalTurns  int                    `json:"total_turns"`
	Bot1Stats   map[string]interface{} `json:"bot1_stats"`
	Bot2Stats   map[string]interface{} `json:"bot2_stats"`
	CompletedAt time.Time              `json:"completed_at"`
}

// Match represents a complete game match between two bots
type Match struct {
	Config     MatchConfig
	GameState  *GameState
	Bot1       *BotPlayer
	Bot2       *BotPlayer
	Replay     *MatchReplay
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// NewMatch creates a new match with the given configuration
func NewMatch(config MatchConfig) (*Match, error) {
	// Create bot players
	bot1, err := NewBotPlayer(1, config.Bot1Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot 1: %w", err)
	}

	bot2, err := NewBotPlayer(2, config.Bot2Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot 2: %w", err)
	}

	// Load map file if provided
	var mapData *Map
	if config.MapPath != "" {
		data, err := os.ReadFile(config.MapPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read map file: %w", err)
		}
		var m Map
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("failed to parse map file: %w", err)
		}
		mapData = &m
		
		// If map contains dimensions, use them (override config dimensions)
		if m.Width > 0 && m.Height > 0 {
			config.GridWidth = m.Width
			config.GridHeight = m.Height
		}
	}

	// Create game state (pass loaded map if any)
	gameState := NewGameState(config.GridWidth, config.GridHeight, mapData)

	// Create context for the match
	// This parent context controls match-level cancellation. Child contexts with timeouts
	// are created per-turn in GetMove() to enforce turn time limits on bot responses.
	ctx, cancel := context.WithCancel(context.Background())

	match := &Match{
		Config:     config,
		GameState:  gameState,
		Bot1:       bot1,
		Bot2:       bot2,
		ctx:        ctx,
		cancelFunc: cancel,
		Replay: &MatchReplay{
			Config: config,
			Turns:  []TurnRecord{},
		},
	}

	// Run automatic calibration on judge host to compute scale relative to
	// the repository reference. This will update tools/reference_calibrate.json
	// when missing, and compute a scale factor = measured_seconds / ref_seconds.
	// If TURN_TIMEOUT_SCALE is not already set in the environment, set it and
	// apply to the configured TurnTimeout so judge matches the reference baseline.
	if err := match.runAutomaticCalibration(); err != nil {
		// Non-fatal: log and continue with configured timeouts
		if match.Config.Verbose {
			fmt.Printf("Calibration warning: %v\n", err)
		}
	}

	// Apply optional turn timeout scaling from environment variable or config
	// Environment variable takes precedence if set: TURN_TIMEOUT_SCALE
	if scaleEnv := os.Getenv("TURN_TIMEOUT_SCALE"); scaleEnv != "" {
		if s, err := strconv.ParseFloat(scaleEnv, 64); err == nil && s > 0 {
			match.Config.TurnTimeout = time.Duration(float64(match.Config.TurnTimeout) * s)
		}
	} else if match.Config.TurnTimeoutScale > 0 {
		match.Config.TurnTimeout = time.Duration(float64(match.Config.TurnTimeout) * match.Config.TurnTimeoutScale)
	}

	return match, nil
}

// Run executes the complete match
func (m *Match) Run() error {
	// Start both bot processes
	if err := m.Bot1.Start(); err != nil {
		return fmt.Errorf("failed to start bot 1: %w", err)
	}
	defer m.Bot1.Stop()

	if err := m.Bot2.Start(); err != nil {
		return fmt.Errorf("failed to start bot 2: %w", err)
	}
	defer m.Bot2.Stop()

	// Give bots a moment to initialize
	time.Sleep(100 * time.Millisecond)

	if m.Config.Verbose {
		fmt.Println("Match started!")
		fmt.Println(m.GameState.String())
	}

	// Main game loop
	for m.GameState.Turn < m.Config.MaxTurns{
		if err := m.PlayTurn(); err != nil {
			return fmt.Errorf("error on turn %d: %w", m.GameState.Turn, err)
		}

		if m.Config.Verbose {
			fmt.Println(m.GameState.String())
		}

		// Small delay between turns for readability
		if m.Config.Verbose {
			time.Sleep(50 * time.Millisecond)
		}

		if m.GameState.GameOver {
			break
		}
	}

	// Record the final state after the last turn
	finalTurnRecord := TurnRecord{
		Turn:       m.GameState.Turn,
		GameState:  m.GameState.Clone(),
		Move1:      "",
		Move2:      "",
		Timeout1:   false,
		Timeout2:   false,
		TimeTaken1: 0,
		TimeTaken2: 0,
	}
	m.Replay.Turns = append(m.Replay.Turns, finalTurnRecord)

	// Finalize match result
	m.finalizeMatch()

	// Save replay
	if err := m.SaveReplay(); err != nil {
		return fmt.Errorf("failed to save replay: %w", err)
	}

	return nil
}

// PlayTurn executes one turn of the game
func (m *Match) PlayTurn() error {
	// Record state before moves
	stateBeforeMove := m.GameState.Clone()

	// Use channels to get moves concurrently
	type moveResult struct {
		botID    int
		response MoveResponse
	}

	moveChan := make(chan moveResult, 2)
	var wg sync.WaitGroup

	// Query both bots simultaneously
	wg.Add(2)

	// Bot 1
	go func() {
		defer wg.Done()
		response := m.Bot1.GetMove(m.ctx, m.GameState, m.Config.TurnTimeout)
		moveChan <- moveResult{botID: 1, response: response}
	}()

	// Bot 2
	go func() {
		defer wg.Done()
		response := m.Bot2.GetMove(m.ctx, m.GameState, m.Config.TurnTimeout)
		moveChan <- moveResult{botID: 2, response: response}
	}()

	// Wait for both responses
	wg.Wait()
	close(moveChan)

	// Collect responses
	var move1Response, move2Response MoveResponse
	for result := range moveChan {
		if result.botID == 1 {
			move1Response = result.response
		} else {
			move2Response = result.response
		}
	}

	// Check if bots are still alive
	if !m.Bot1.IsAlive() && m.GameState.Snakes[0].Alive {
		m.GameState.Snakes[0].Alive = false
		if m.Config.Verbose {
			fmt.Println("Bot 1 process died!")
		}
	}

	if !m.Bot2.IsAlive() && m.GameState.Snakes[1].Alive {
		m.GameState.Snakes[1].Alive = false
		if m.Config.Verbose {
			fmt.Println("Bot 2 process died!")
		}
	}

	// Process the turn with both moves
	m.GameState.ProcessTurn(move1Response.Move, move2Response.Move)

	// Record the turn
	turnRecord := TurnRecord{
		Turn:       m.GameState.Turn,
		GameState:  stateBeforeMove,
		Move1:      move1Response.Move,
		Move2:      move2Response.Move,
		Timeout1:   move1Response.Timeout,
		Timeout2:   move2Response.Timeout,
		TimeTaken1: move1Response.TimeTaken,
		TimeTaken2: move2Response.TimeTaken,
	}
	m.Replay.Turns = append(m.Replay.Turns, turnRecord)

	if m.Config.Verbose && (move1Response.Timeout || move2Response.Timeout) {
		if move1Response.Timeout {
			fmt.Printf("Bot 1 timeout! (Total: %d)\n", m.Bot1.timeoutCount)
		}
		if move2Response.Timeout {
			fmt.Printf("Bot 2 timeout! (Total: %d)\n", m.Bot2.timeoutCount)
		}
	}

	return nil
}

// finalizeMatch determines the final winner and reason
func (m *Match) finalizeMatch() {
	m.Replay.TotalTurns = m.GameState.Turn
	m.Replay.Winner = m.GameState.Winner
	m.Replay.CompletedAt = time.Now()
	m.Replay.Bot1Stats = m.Bot1.GetStats()
	m.Replay.Bot2Stats = m.Bot2.GetStats()

	// Determine win reason with specific death causes
	if m.GameState.Winner == 0 {
		if m.GameState.Turn >= m.Config.MaxTurns {
			m.Replay.WinReason = "Draw - Max turns reached"
			// Award win to longer snake
			if m.GameState.Snakes[0].Length > m.GameState.Snakes[1].Length {
				m.Replay.Winner = 1
				m.Replay.WinReason = "Bot 1 wins - Longer snake at max turns"
			} else if m.GameState.Snakes[1].Length > m.GameState.Snakes[0].Length {
				m.Replay.Winner = 2
				m.Replay.WinReason = "Bot 2 wins - Longer snake at max turns"
			}
		} else {
			// Both died - describe how
			reason1 := m.getDeathDescription(m.GameState.Snakes[0].DeathReason)
			reason2 := m.getDeathDescription(m.GameState.Snakes[1].DeathReason)
			m.Replay.WinReason = fmt.Sprintf("Draw - Both snakes died (Bot 1: %s, Bot 2: %s)", reason1, reason2)
		}
	} else if m.GameState.Winner == 1 {
		if !m.GameState.Snakes[1].Alive {
			reason := m.getDeathDescription(m.GameState.Snakes[1].DeathReason)
			m.Replay.WinReason = fmt.Sprintf("Bot 1 wins - Bot 2 died due to %s", reason)
		} else {
			m.Replay.WinReason = "Bot 1 wins"
		}
	} else if m.GameState.Winner == 2 {
		if !m.GameState.Snakes[0].Alive {
			reason := m.getDeathDescription(m.GameState.Snakes[0].DeathReason)
			m.Replay.WinReason = fmt.Sprintf("Bot 2 wins - Bot 1 died due to %s", reason)
		} else {
			m.Replay.WinReason = "Bot 2 wins"
		}
	}

	fmt.Printf("\n=== Match Complete ===\n")
	fmt.Printf("Winner: %s\n", m.Replay.WinReason)
	fmt.Printf("Total Turns: %d\n", m.Replay.TotalTurns)
	fmt.Printf("Bot 1 - Timeouts: %d, Errors: %d, Final Length: %d\n",
		m.Bot1.timeoutCount, m.Bot1.errorCount, m.GameState.Snakes[0].Length)
	fmt.Printf("Bot 2 - Timeouts: %d, Errors: %d, Final Length: %d\n",
		m.Bot2.timeoutCount, m.Bot2.errorCount, m.GameState.Snakes[1].Length)
}

// getDeathDescription converts a death reason code to a human-readable description
func (m *Match) getDeathDescription(deathReason string) string {
	switch deathReason {
	case "wall":
		return "collision with wall"
	case "self":
		return "self-collision"
	case "body":
		return "collision with larger snake"
	case "head-to-head":
		return "head-to-head collision with larger snake"
	case "hunger":
		return "hunger"
	case "obstacle":
		return "collision with obstacle"
	default:
		return "unknown cause"
	}
}

// SaveReplay writes the match replay to a JSON file
func (m *Match) SaveReplay() error {
	replayPath := m.Config.ReplayOutput
	if replayPath == "" {
		replayPath = "replays/match_replay.json"
	}

	file, err := os.Create(replayPath)
	if err != nil {
		return fmt.Errorf("failed to create replay file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(m.Replay); err != nil {
		return fmt.Errorf("failed to encode replay: %w", err)
	}

	if m.Config.Verbose {
		fmt.Printf("Replay saved to: %s\n", replayPath)
	}

	return nil
}

// Stop gracefully stops the match
func (m *Match) Stop() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
	if m.Bot1 != nil {
		m.Bot1.Stop()
	}
	if m.Bot2 != nil {
		m.Bot2.Stop()
	}
}

// runAutomaticCalibration runs the Python calibrator and updates the
// tools/reference_calibrate.json file. If a repository reference exists it will
// compute a scale factor = measured_seconds / ref_seconds and, when
// TURN_TIMEOUT_SCALE is not already set, export the value and apply it to the
// match's TurnTimeout in NewMatch.
func (m *Match) runAutomaticCalibration() error {
	// Try common python executables
	cmds := [][]string{{"python", "tools/calibrate.py"}, {"python3", "tools/calibrate.py"}, {"py", "tools/calibrate.py"}}
	var out []byte
	var err error
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = ""
		out, err = cmd.Output()
		if err == nil {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("failed to run calibrator: %w", err)
	}

	var res calibrateResult
	if err := json.Unmarshal(out, &res); err != nil {
		return fmt.Errorf("failed to parse calibrator output: %w", err)
	}

	// Read existing reference if present
	refPath := "tools/reference_calibrate.json"
	var refSeconds float64
	if data, err := os.ReadFile(refPath); err == nil {
		var parsed struct {
			RefSeconds float64 `json:"ref_seconds"`
		}
		if err := json.Unmarshal(data, &parsed); err == nil && parsed.RefSeconds > 0 {
			refSeconds = parsed.RefSeconds
		}
	}

	// If no reference exists, write measured as the new reference
	if refSeconds == 0 {
		outData, _ := json.MarshalIndent(map[string]float64{"ref_seconds": res.Seconds}, "", "  ")
		if err := os.WriteFile(refPath, outData, 0644); err != nil {
			return fmt.Errorf("failed to write reference calibrate file: %w", err)
		}
		if m.Config.Verbose {
			fmt.Printf("Wrote new reference calibrate value: %f (to %s)\n", res.Seconds, refPath)
		}
		refSeconds = res.Seconds
	}

	// Compute scale relative to the reference: measured_seconds / ref_seconds
	scale := 1.0
	if refSeconds > 0 {
		scale = res.Seconds / refSeconds
	}

	// If TURN_TIMEOUT_SCALE is not set in environment, set it so bot children
	// inherit it when started. Also leave it for NewMatch to apply to timeouts.
	if os.Getenv("TURN_TIMEOUT_SCALE") == "" {
		if err := os.Setenv("TURN_TIMEOUT_SCALE", strconv.FormatFloat(scale, 'f', 6, 64)); err != nil {
			return fmt.Errorf("failed to set TURN_TIMEOUT_SCALE env: %w", err)
		}
		if m.Config.Verbose {
			fmt.Printf("Calibration: measured=%f ref=%f scale=%f (TURN_TIMEOUT_SCALE set)\n", res.Seconds, refSeconds, scale)
		}
	}

	return nil
}
