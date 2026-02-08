package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BotConfig represents the configuration file for a bot
type BotConfig struct {
	Command []string `json:"command"` // e.g., ["python3", "bot.py"] or ["./java_bot"]
	Name    string   `json:"name"`    // Optional display name
	// Optional Docker image to run the bot inside. If set, the judge will
	// execute `docker run` with the image and communicate over stdin/stdout.
	DockerImage string `json:"docker_image"`
	// Optional Docker CPU quota (e.g. 0.5) and memory (e.g. "256m")
	DockerCPUs   float64 `json:"docker_cpus"`
	DockerMemory string  `json:"docker_memory"`
}

// BotPlayer manages a single bot process
type BotPlayer struct {
	ID           int
	Directory    string
	Config       BotConfig
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	stdout       io.ReadCloser
	stderr       io.ReadCloser
	scanner      *bufio.Scanner
	isRunning    bool
	timeoutCount int
	errorCount   int
}

// MoveResponse represents a bot's response
type MoveResponse struct {
	Move      Direction     `json:"move"`
	Timeout   bool          `json:"timeout"`
	Error     error         `json:"error,omitempty"`
	TimeTaken time.Duration `json:"time_taken"`
}

// LoadBotConfig reads the config.json from a bot directory
func LoadBotConfig(directory string) (*BotConfig, error) {
	configPath := filepath.Join(directory, "config.json")
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config.json: %w", err)
	}
	defer file.Close()

	var config BotConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config.json: %w", err)
	}

	if len(config.Command) == 0 {
		return nil, fmt.Errorf("command array is empty in config.json")
	}

	return &config, nil
}

// NewBotPlayer creates a new bot player
func NewBotPlayer(id int, directory string) (*BotPlayer, error) {
	config, err := LoadBotConfig(directory)
	if err != nil {
		return nil, err
	}

	return &BotPlayer{
		ID:        id,
		Directory: directory,
		Config:    *config,
		isRunning: false,
	}, nil
}

// Start launches the bot process
func (bp *BotPlayer) Start() error {
	if bp.isRunning {
		return fmt.Errorf("bot %d is already running", bp.ID)
	}

	// Prepare command
	var cmdName string
	var cmdArgs []string

	// If a Docker image is specified in the bot config, run the bot inside Docker.
	// Use -i so stdin/stdout can be attached to the docker process.
	if bp.Config.DockerImage != "" {
		fmt.Printf("[Bot %d] Running in Docker: %s (CPUs: %.1f, Memory: %s)\n", bp.ID, bp.Config.DockerImage, bp.Config.DockerCPUs, bp.Config.DockerMemory)
		cmdName = "docker"
		// Build docker args: run --rm -i [--cpus X] [--memory Y] image
		cmdArgs = []string{"run", "--rm", "-i"}
		if bp.Config.DockerCPUs > 0 {
			cmdArgs = append(cmdArgs, "--cpus", fmt.Sprintf("%g", bp.Config.DockerCPUs))
		}
		if bp.Config.DockerMemory != "" {
			cmdArgs = append(cmdArgs, "--memory", bp.Config.DockerMemory)
		}
		// Mount the bot directory into /bot inside the container so scripts/tools are accessible
		// Only do this if a directory exists
		if bp.Directory != "" {
			absDir, err := filepath.Abs(bp.Directory)
			if err == nil {
				// mount as read-only to avoid accidental modification
				cmdArgs = append(cmdArgs, "-v", fmt.Sprintf("%s:/bot:ro", absDir))
			}
		}
		cmdArgs = append(cmdArgs, bp.Config.DockerImage)
	} else {
		fmt.Printf("[Bot %d] Running locally: %v\n", bp.ID, bp.Config.Command)
		cmdName = bp.Config.Command[0]
		cmdArgs = bp.Config.Command[1:]
	}

	bp.cmd = exec.Command(cmdName, cmdArgs...)
	bp.cmd.Dir = bp.Directory

	// Setup pipes
	stdin, err := bp.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	bp.stdin = stdin

	stdout, err := bp.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	bp.stdout = stdout

	stderr, err := bp.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	bp.stderr = stderr

	// Start the process
	if err := bp.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start bot process: %w", err)
	}

	// Setup scanner for stdout
	bp.scanner = bufio.NewScanner(bp.stdout)
	bp.isRunning = true

	// Log stderr in background
	go bp.logStderr()

	return nil
}

// logStderr logs stderr output from the bot (for debugging)
func (bp *BotPlayer) logStderr() {
	// Create log file in bot directory
	logPath := filepath.Join(bp.Directory, fmt.Sprintf("bot_%d_stderr.log", bp.ID))
	logFile, err := os.Create(logPath)
	if err != nil {
		// If we can't create log file, silently consume stderr
		scanner := bufio.NewScanner(bp.stderr)
		for scanner.Scan() {
			_ = scanner.Text()
		}
		return
	}
	defer logFile.Close()

	scanner := bufio.NewScanner(bp.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(logFile, line)
	}
}

// GetMove sends game state to bot and waits for a move with timeout
func (bp *BotPlayer) GetMove(ctx context.Context, gameState *GameState, timeout time.Duration) MoveResponse {
	if !bp.isRunning {
		return MoveResponse{
			Move:    bp.getDefaultMove(gameState),
			Timeout: false,
			Error:   fmt.Errorf("bot is not running"),
		}
	}

	startTime := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Send game state to bot with reordered snakes so bot's snake is at index 0
	gameStateJSON, err := gameState.ToJSON(bp.ID)
	if err != nil {
		bp.errorCount++
		return MoveResponse{
			Move:    bp.getDefaultMove(gameState),
			Timeout: false,
			Error:   fmt.Errorf("failed to serialize game state: %w", err),
		}
	}

	// Channel to receive the move
	moveChan := make(chan MoveResponse, 1)

	// Goroutine to send state and read response
	go func() {
		// Send game state
		_, err := bp.stdin.Write(append(gameStateJSON, '\n'))
		if err != nil {
			moveChan <- MoveResponse{
				Move:  bp.getDefaultMove(gameState),
				Error: fmt.Errorf("failed to write to bot: %w", err),
			}
			return
		}

		// Read response
		if bp.scanner.Scan() {
			line := strings.TrimSpace(bp.scanner.Text())

			var response struct {
				Move string `json:"move"`
			}

			err := json.Unmarshal([]byte(line), &response)
			if err != nil {
				// Try parsing as plain string
				response.Move = strings.ToUpper(line)
			}

			move := bp.parseMove(response.Move)
			moveChan <- MoveResponse{
				Move:      move,
				Timeout:   false,
				Error:     nil,
				TimeTaken: time.Since(startTime),
			}
		} else {
			// Scanner error or EOF
			moveChan <- MoveResponse{
				Move:  bp.getDefaultMove(gameState),
				Error: fmt.Errorf("failed to read from bot"),
			}
		}
	}()

	// Wait for move or timeout
	select {
	case response := <-moveChan:
		return response
	case <-ctx.Done():
		bp.timeoutCount++
		return MoveResponse{
			Move:      bp.getDefaultMove(gameState),
			Timeout:   true,
			Error:     fmt.Errorf("bot timeout"),
			TimeTaken: timeout,
		}
	}
}

// parseMove converts string move to Direction
func (bp *BotPlayer) parseMove(moveStr string) Direction {
	moveStr = strings.ToUpper(strings.TrimSpace(moveStr))

	switch moveStr {
	case "UP", "U", "W":
		return DirectionUp
	case "DOWN", "D", "S":
		return DirectionDown
	case "LEFT", "L", "A":
		return DirectionLeft
	case "RIGHT", "R":
		return DirectionRight
	default:
		// Invalid move, use current direction
		return DirectionUp // Default fallback
	}
}

// getDefaultMove returns a safe default move (continues current direction)
func (bp *BotPlayer) getDefaultMove(gameState *GameState) Direction {
	// Get the snake's current direction
	snake := gameState.Snakes[bp.ID-1]
	return snake.Direction
}

// Stop terminates the bot process
func (bp *BotPlayer) Stop() error {
	if !bp.isRunning {
		return nil
	}

	bp.isRunning = false

	// Close stdin to signal bot to exit
	if bp.stdin != nil {
		bp.stdin.Close()
	}

	// Give the process a moment to exit gracefully
	time.Sleep(100 * time.Millisecond)

	// Kill if still running
	if bp.cmd != nil && bp.cmd.Process != nil {
		bp.cmd.Process.Kill()
		bp.cmd.Wait()
	}

	return nil
}

// GetStats returns statistics about the bot's performance
func (bp *BotPlayer) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"id":            bp.ID,
		"name":          bp.Config.Name,
		"timeout_count": bp.timeoutCount,
		"error_count":   bp.errorCount,
	}
}

// IsAlive checks if the bot process is still running
func (bp *BotPlayer) IsAlive() bool {
	if !bp.isRunning || bp.cmd == nil || bp.cmd.Process == nil {
		return false
	}

	// Simply check if isRunning flag is set
	// The flag is cleared when Stop() is called or process exits
	return bp.isRunning
}
