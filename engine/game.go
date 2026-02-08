package engine

import (
	"encoding/json"
	"fmt"
	"math/rand"
)

// Direction represents movement direction
type Direction string

const (
	DirectionUp    Direction = "UP"
	DirectionDown  Direction = "DOWN"
	DirectionLeft  Direction = "LEFT"
	DirectionRight Direction = "RIGHT"
)

// AppleType represents the type of apple
type AppleType string

const (
	AppleNormal AppleType = "NORMAL" // Regular apple: +1 length

	AppleGod    AppleType = "GOD"    // God apple: +3 points (counts as 3 length)
	AppleSpeed  AppleType = "SPEED"  // Speed apple: 2 moves per turn for next 5 turns
	AppleSleep  AppleType = "SLEEP"  // Sleep apple: freeze opponent for 5 turns
	ApplePoison AppleType = "POISON" // Poison apple: -1 length and -1 score
)

// Position represents a coordinate on the grid
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Apple represents an apple on the game grid
type Apple struct {
	X    int       `json:"x"`
	Y    int       `json:"y"`
	Type AppleType `json:"type"`
}

// Snake represents a player's snake
type Snake struct {
	ID          int        `json:"id"`
	Body        []Position `json:"body"` // Head is at index 0
	Direction   Direction  `json:"direction"`
	Alive       bool       `json:"alive"`
	Length      int        `json:"length"`
	Score       int        `json:"score"`        // Score from eating apples
	SpeedTurns  int        `json:"speed_turns"`  // Remaining turns with 2x speed
	SleepTurns  int        `json:"sleep_turns"`  // Remaining turns frozen
	Energy      int        `json:"energy"`       // Energy depletes by 1 per turn, eating apple restores to 60
	DeathReason string     `json:"death_reason"` // Reason for death: "wall", "self", "body", "head-to-head", "hunger", "obstacle"
}

// GameState represents the complete state of the game
type GameState struct {
	Turn       int       `json:"turn"`
	GridWidth  int       `json:"grid_width"`
	GridHeight int       `json:"grid_height"`
	Snakes     [2]*Snake `json:"snakes"`
	Apples     []Apple   `json:"apples"`
	Map        *Map      `json:"map"`
	Winner     int       `json:"winner"` // 0 = no winner yet, 1 or 2 = winner
	GameOver   bool      `json:"game_over"`
}

// Map represents static map data like obstacles
type Map struct {
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	Obstacles []Position `json:"obstacles"`
}

// NewGameState creates a new game with initial snake positions
// The optional param m can be nil for an empty map
func NewGameState(width, height int, m *Map) *GameState {
	gs := &GameState{
		Turn:       0,
		GridWidth:  width,
		GridHeight: height,
		Apples:     []Apple{},
		Map:        m,
		Winner:     0,
		GameOver:   false,
	}

	// Initialize Snake 1 (top-left corner)
	gs.Snakes[0] = &Snake{
		ID: 1,
		Body: []Position{
			{X: 1, Y: 2},
			{X: 1, Y: 1},
			{X: 1, Y: 0},
		},
		Direction:   DirectionDown,
		Alive:       true,
		Length:      3,
		Score:       0,
		SpeedTurns:  0,
		SleepTurns:  0,
		Energy:      60,
		DeathReason: "",
	}

	// Initialize Snake 2 (bottom-right corner)
	gs.Snakes[1] = &Snake{
		ID: 2,
		Body: []Position{
			{X: width - 2, Y: height - 3},
			{X: width - 2, Y: height - 2},
			{X: width - 2, Y: height - 1},
		},
		Direction:   DirectionUp,
		Alive:       true,
		Length:      3,
		Score:       0,
		SpeedTurns:  0,
		SleepTurns:  0,
		Energy:      60,
		DeathReason: "",
	}

	// Spawn initial apples
	gs.SpawnApple()
	gs.SpawnApple()
	gs.SpawnApple()

	return gs
}

// ToJSON converts the game state to JSON for bot communication
// botID indicates which bot is receiving this state (1 or 2)
// The snakes array is reordered so the receiving bot's snake is always at index 0
func (gs *GameState) ToJSON(botID int) ([]byte, error) {
	// Create a copy of the game state
	stateForBot := *gs

	// Reorder snakes so the receiving bot's snake is at index 0
	if botID == 2 {
		// Bot 2: swap order [snake2, snake1]
		stateForBot.Snakes[0] = gs.Snakes[1]
		stateForBot.Snakes[1] = gs.Snakes[0]
	}
	// Bot 1: original order is already correct

	return json.Marshal(stateForBot)
}

// GetHead returns the head position of a snake
func (s *Snake) GetHead() Position {
	if len(s.Body) == 0 {
		return Position{X: -1, Y: -1}
	}
	return s.Body[0]
}

// Move updates the snake position based on direction
func (s *Snake) Move(newDirection Direction, grow bool) {
	if !s.Alive {
		return
	}

	// validity check for 180-degree turn
	if s.isNot180(newDirection) {
		s.Direction = newDirection
	}

	// Calculate new head position
	head := s.GetHead()
	newHead := s.getNextPosition(head, s.Direction)

	// Add new head
	s.Body = append([]Position{newHead}, s.Body...) //new slice with existing body "spread" afterwards

	// Remove tail if not growing
	if !grow {
		s.Body = s.Body[:len(s.Body)-1]
	} else {
		s.Length++
	}
}

// isNot180 prevents 180-degree turns
func (s *Snake) isNot180(newDir Direction) bool {
	if len(s.Body) < 2 {
		return true
	}

	//good one, Copilot
	opposite := map[Direction]Direction{
		DirectionUp:    DirectionDown,
		DirectionDown:  DirectionUp,
		DirectionLeft:  DirectionRight,
		DirectionRight: DirectionLeft,
	}

	return opposite[s.Direction] != newDir
}

// getNextPosition calculates the next position based on direction
func (s *Snake) getNextPosition(pos Position, dir Direction) Position {
	switch dir {
	case DirectionUp:
		return Position{X: pos.X, Y: pos.Y - 1}
	case DirectionDown:
		return Position{X: pos.X, Y: pos.Y + 1}
	case DirectionLeft:
		return Position{X: pos.X - 1, Y: pos.Y}
	case DirectionRight:
		return Position{X: pos.X + 1, Y: pos.Y}
	}
	return pos
}

// CheckCollision checks if snake collided with walls, itself, or other snake
func (gs *GameState) CheckCollision(snakeID int) bool {
	snake := gs.Snakes[snakeID-1]
	if !snake.Alive {
		return false
	}

	head := snake.GetHead()

	// Check wall collision
	if head.X < 0 || head.X >= gs.GridWidth || head.Y < 0 || head.Y >= gs.GridHeight {
		snake.Alive = false
		snake.DeathReason = "wall"
		return true
	}

	// Check self-collision (skip the head itself)
	for i := 1; i < len(snake.Body); i++ {
		if head.X == snake.Body[i].X && head.Y == snake.Body[i].Y {
			snake.Alive = false
			snake.DeathReason = "self"
			return true
		}
	}

	// Check collision with other snake's body (skip head for head-to-head check)
	otherSnake := gs.Snakes[(snakeID % 2)] // 1->0, 2->1
	for i, segment := range otherSnake.Body {
		if i == 0 {
			continue // Skip head, handled by head-to-head collision check
		}
		if head.X == segment.X && head.Y == segment.Y {
			snake.Alive = false
			snake.DeathReason = "body"
			return true
		}
	}

	// Check collision with map obstacles
	if gs.Map != nil {
		for _, obs := range gs.Map.Obstacles {
			if head.X == obs.X && head.Y == obs.Y {
				snake.Alive = false
				snake.DeathReason = "obstacle"
				return true
			}
		}
	}

	return false
}

// CheckAppleEaten checks if snake ate an apple and returns the apple type
func (gs *GameState) CheckAppleEaten(snakeID int) (bool, AppleType) {
	snake := gs.Snakes[snakeID-1]
	if !snake.Alive {
		return false, ""
	}

	head := snake.GetHead()

	for i, apple := range gs.Apples {
		if head.X == apple.X && head.Y == apple.Y {
			// Remove eaten apple
			appleType := apple.Type
			gs.Apples = append(gs.Apples[:i], gs.Apples[i+1:]...)
			return true, appleType
		}
	}

	return false, ""
}

// manhattanDistance calculates the Manhattan distance between two positions
func manhattanDistance(p1, p2 Position) int {
	dx := p1.X - p2.X
	if dx < 0 {
		dx = -dx
	}
	dy := p1.Y - p2.Y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// SpawnApple spawns a new apple using zone-based balanced spawning
// Positions are categorized as: Snake1 territory, Snake2 territory, or Neutral
// This ensures fair distribution and reduces spawn RNG
func (gs *GameState) SpawnApple() {
	// Build set of occupied positions
	occupied := make(map[Position]bool)

	// Add snake segments
	for _, snake := range gs.Snakes {
		for _, segment := range snake.Body {
			occupied[segment] = true
		}
	}

	// Add existing apples
	for _, apple := range gs.Apples {
		occupied[Position{X: apple.X, Y: apple.Y}] = true
	}

	// Add map obstacles
	if gs.Map != nil {
		for _, obs := range gs.Map.Obstacles {
			occupied[obs] = true
		}
	}

	// Get snake heads for distance calculation
	head1 := gs.Snakes[0].GetHead()
	head2 := gs.Snakes[1].GetHead()

	// Categorize all empty positions into zones
	var snake1Positions []Position  // Closer to snake 1
	var snake2Positions []Position  // Closer to snake 2
	var neutralPositions []Position // Roughly equidistant (within 3 tiles)

	for y := 0; y < gs.GridHeight; y++ {
		for x := 0; x < gs.GridWidth; x++ {
			pos := Position{X: x, Y: y}
			if !occupied[pos] {
				dist1 := manhattanDistance(pos, head1)
				dist2 := manhattanDistance(pos, head2)
				distDiff := dist1 - dist2
				if distDiff < 0 {
					distDiff = -distDiff
				}

				if distDiff <= 3 {
					// Roughly equidistant (within 3 tiles difference)
					neutralPositions = append(neutralPositions, pos)
				} else if dist1 < dist2 {
					snake1Positions = append(snake1Positions, pos)
				} else {
					snake2Positions = append(snake2Positions, pos)
				}
			}
		}
	}

	// Count existing apples in each zone to balance spawning
	snake1AppleCount := 0
	snake2AppleCount := 0
	neutralAppleCount := 0

	for _, apple := range gs.Apples {
		applePos := Position{X: apple.X, Y: apple.Y}
		dist1 := manhattanDistance(applePos, head1)
		dist2 := manhattanDistance(applePos, head2)
		distDiff := dist1 - dist2
		if distDiff < 0 {
			distDiff = -distDiff
		}

		if distDiff <= 3 {
			neutralAppleCount++
		} else if dist1 < dist2 {
			snake1AppleCount++
		} else {
			snake2AppleCount++
		}
	}

	// Select zone to spawn in based on current distribution
	// Priority: spawn in zone with fewest apples for balance
	var selectedPositions []Position

	// Find the zone with minimum apple count
	selectedPositions = neutralPositions

	if snake2AppleCount < snake1AppleCount && len(snake2Positions) > 0 {
		selectedPositions = snake2Positions
	} else if snake1AppleCount < snake2AppleCount && len(snake1Positions) > 0 {
		selectedPositions = snake1Positions
	}

	// Fallback: if selected zone is empty, try other zones
	if len(selectedPositions) == 0 {
		if len(neutralPositions) > 0 {
			selectedPositions = neutralPositions
		} else if len(snake1Positions) > 0 {
			selectedPositions = snake1Positions
		} else if len(snake2Positions) > 0 {
			selectedPositions = snake2Positions
		}
	}

	// Spawn apple in selected zone
	if len(selectedPositions) > 0 {
		randomIndex := rand.Intn(len(selectedPositions))
		pos := selectedPositions[randomIndex]

		// Randomly select apple type (weighted distribution)
		// 60% NORMAL, 15% GOD, 15% SPEED, 5% SLEEP, 5% POISON
		rng := rand.Intn(100)
		appleType := AppleNormal
		if rng < 60 {
			appleType = AppleNormal
		} else if rng < 75 {
			appleType = AppleGod
		} else if rng < 90 {
			appleType = AppleSpeed
		} else if rng < 95 {
			appleType = AppleSleep
		} else {
			appleType = ApplePoison
		}

		gs.Apples = append(gs.Apples, Apple{X: pos.X, Y: pos.Y, Type: appleType})
	}
}

// checkGameOver determines if the game is over and sets winner
func (gs *GameState) checkGameOver() {
	snake1Alive := gs.Snakes[0].Alive
	snake2Alive := gs.Snakes[1].Alive

	if !snake1Alive && !snake2Alive {
		gs.GameOver = true
		// Determine winner by length
		if gs.Snakes[0].Length > gs.Snakes[1].Length {
			gs.Winner = 1
		} else if gs.Snakes[1].Length > gs.Snakes[0].Length {
			gs.Winner = 2
		} else {
			gs.Winner = 0 // Draw
		}
	} else if !snake1Alive {
		gs.GameOver = true
		gs.Winner = 2
	} else if !snake2Alive {
		gs.GameOver = true
		gs.Winner = 1
	}
}

// ProcessTurn processes one turn of the game
func (gs *GameState) ProcessTurn(move1, move2 Direction) {
	gs.Turn++

	// Move snakes (once or twice if speed is active)
	// Each snake moves independently based on their speed state
	movesToMake1 := 1
	movesToMake2 := 1
	if gs.Snakes[0].SpeedTurns > 0 {
		movesToMake1 = 2
	}
	if gs.Snakes[1].SpeedTurns > 0 {
		movesToMake2 = 2
	}

	// Determine max moves needed for the loop
	maxMoves := movesToMake1
	if movesToMake2 > maxMoves {
		maxMoves = movesToMake2
	}

	for moveCount := 0; moveCount < maxMoves; moveCount++ {
		// Only move if not frozen and still have moves remaining
		if gs.Snakes[0].SleepTurns <= 0 && moveCount < movesToMake1 {
			gs.Snakes[0].Move(move1, false)
		}
		if gs.Snakes[1].SleepTurns <= 0 && moveCount < movesToMake2 {
			gs.Snakes[1].Move(move2, false)
		}

		// Check for collisions
		collision1 := gs.CheckCollision(1)
		collision2 := gs.CheckCollision(2)

		// Check for head-to-head collision
		if gs.Snakes[0].Alive && gs.Snakes[1].Alive {
			head1 := gs.Snakes[0].GetHead()
			head2 := gs.Snakes[1].GetHead()
			if head1.X == head2.X && head1.Y == head2.Y {
				// Head-to-head collision: smaller snake dies, equal length = both die
				if gs.Snakes[0].Length > gs.Snakes[1].Length {
					// Snake 1 is longer, Snake 2 dies
					gs.Snakes[1].Alive = false
					gs.Snakes[1].DeathReason = "head-to-head"
					collision2 = true
				} else if gs.Snakes[1].Length > gs.Snakes[0].Length {
					// Snake 2 is longer, Snake 1 dies
					gs.Snakes[0].Alive = false
					gs.Snakes[0].DeathReason = "head-to-head"
					collision1 = true
				} else {
					// Equal length: both die
					gs.Snakes[0].Alive = false
					gs.Snakes[0].DeathReason = "head-to-head"
					gs.Snakes[1].Alive = false
					gs.Snakes[1].DeathReason = "head-to-head"
					collision1 = true
					collision2 = true
				}
			}
		}

		if collision1 || collision2 {
			break // Stop if collision occurs
		}
	}

	// Track which snakes ate apples this turn (to skip energy depletion)
	snake1AteApple := false
	snake2AteApple := false

	// Check for apple consumption and apply effects
	if gs.Snakes[0].Alive {
		eaten, appleType := gs.CheckAppleEaten(1)
		if eaten {
			gs.ApplyAppleEffect(1, appleType)
			gs.SpawnApple()
			snake1AteApple = true
		}
	}

	if gs.Snakes[1].Alive {
		eaten, appleType := gs.CheckAppleEaten(2)
		if eaten {
			gs.ApplyAppleEffect(2, appleType)
			gs.SpawnApple()
			snake2AteApple = true
		}
	}

	// Decrement speed and sleep timers AFTER movement and effects
	if gs.Snakes[0].SpeedTurns > 0 {
		gs.Snakes[0].SpeedTurns--
	}
	if gs.Snakes[1].SpeedTurns > 0 {
		gs.Snakes[1].SpeedTurns--
	}
	if gs.Snakes[0].SleepTurns > 0 {
		gs.Snakes[0].SleepTurns--
	}
	if gs.Snakes[1].SleepTurns > 0 {
		gs.Snakes[1].SleepTurns--
	}

	// Deplete energy by 1 per turn for alive snakes (but not if they ate an apple this turn)
	if gs.Snakes[0].Alive && !snake1AteApple {
		gs.Snakes[0].Energy--
		if gs.Snakes[0].Energy <= 0 {
			gs.Snakes[0].Alive = false
			gs.Snakes[0].DeathReason = "hunger"
		}
	}
	if gs.Snakes[1].Alive && !snake2AteApple {
		gs.Snakes[1].Energy--
		if gs.Snakes[1].Energy <= 0 {
			gs.Snakes[1].Alive = false
			gs.Snakes[1].DeathReason = "hunger"
		}
	}

	// Check game over conditions
	gs.checkGameOver()
}

// ApplyAppleEffect applies the effect of eating an apple
func (gs *GameState) ApplyAppleEffect(snakeID int, appleType AppleType) {
	snake := gs.Snakes[snakeID-1]
	otherSnakeID := 3 - snakeID // 1 -> 2, 2 -> 1
	otherSnake := gs.Snakes[otherSnakeID-1]

	// Restore energy to 60 when eating any apple
	snake.Energy = 60

	switch appleType {
	case AppleNormal:
		// Standard growth
		snake.Length++
		snake.Score++
		// Add segment to tail
		if len(snake.Body) > 0 {
			tail := snake.Body[len(snake.Body)-1]
			snake.Body = append(snake.Body, tail)
		}

	case AppleGod:
		// Worth 3 points
		snake.Length += 3
		snake.Score += 3
		// Add 3 segments to tail
		if len(snake.Body) > 0 {
			tail := snake.Body[len(snake.Body)-1]
			snake.Body = append(snake.Body, tail)
			snake.Body = append(snake.Body, tail)
			snake.Body = append(snake.Body, tail)
		}

	case AppleSpeed:
		// 2 moves per turn for 5 turns
		snake.Length++
		snake.Score++
		snake.SpeedTurns = 5
		// Add segment to tail
		if len(snake.Body) > 0 {
			tail := snake.Body[len(snake.Body)-1]
			snake.Body = append(snake.Body, tail)
		}

	case AppleSleep:
		// Freeze opponent for 5 turns
		otherSnake.SleepTurns = 5
		snake.Length++
		snake.Score++
		// Add segment to tail
		if len(snake.Body) > 0 {
			tail := snake.Body[len(snake.Body)-1]
			snake.Body = append(snake.Body, tail)
		}

	case ApplePoison:
		// Reduce length and score by 1
		if snake.Length > 1 {
			snake.Length--
			if len(snake.Body) > 0 {
				snake.Body = snake.Body[:len(snake.Body)-1]
			}
		}
		if snake.Score > 0 {
			snake.Score--
		}
	}
}

// Clone creates a deep copy of the game state for the turn history
func (gs *GameState) Clone() *GameState {
	clone := &GameState{
		Turn:       gs.Turn,
		GridWidth:  gs.GridWidth,
		GridHeight: gs.GridHeight,
		Winner:     gs.Winner,
		GameOver:   gs.GameOver,
	}

	// Clone snakes
	for i := range gs.Snakes {
		clone.Snakes[i] = &Snake{
			ID:          gs.Snakes[i].ID,
			Direction:   gs.Snakes[i].Direction,
			Alive:       gs.Snakes[i].Alive,
			Length:      gs.Snakes[i].Length,
			Score:       gs.Snakes[i].Score,
			SpeedTurns:  gs.Snakes[i].SpeedTurns,
			SleepTurns:  gs.Snakes[i].SleepTurns,
			Energy:      gs.Snakes[i].Energy,
			DeathReason: gs.Snakes[i].DeathReason,
			Body:        make([]Position, len(gs.Snakes[i].Body)),
		}
		copy(clone.Snakes[i].Body, gs.Snakes[i].Body)
	}

	// Clone apples
	clone.Apples = make([]Apple, len(gs.Apples))
	copy(clone.Apples, gs.Apples)

	// Clone map
	if gs.Map != nil {
		clone.Map = &Map{
			Obstacles: make([]Position, len(gs.Map.Obstacles)),
		}
		copy(clone.Map.Obstacles, gs.Map.Obstacles)
	}

	return clone
}

// String returns a visual representation of the game state
func (gs *GameState) String() string {
	grid := make([][]rune, gs.GridHeight)
	for i := range grid {
		grid[i] = make([]rune, gs.GridWidth)
		for j := range grid[i] {
			grid[i][j] = '.'
		}
	}

	// Draw apples with different symbols based on type
	for _, apple := range gs.Apples {
		if apple.Y >= 0 && apple.Y < gs.GridHeight && apple.X >= 0 && apple.X < gs.GridWidth {
			symbol := 'A'
			switch apple.Type {
			case AppleNormal:
				symbol = 'A'
			case AppleGod:
				symbol = 'D'
			case AppleSpeed:
				symbol = 'S'
			case AppleSleep:
				symbol = 'Z'
			case ApplePoison:
				symbol = 'P'
			}
			grid[apple.Y][apple.X] = symbol
		}
	}

	// Draw map obstacles (if any)
	if gs.Map != nil {
		for _, obs := range gs.Map.Obstacles {
			if obs.Y >= 0 && obs.Y < gs.GridHeight && obs.X >= 0 && obs.X < gs.GridWidth {
				grid[obs.Y][obs.X] = '#'
			}
		}
	}

	// Draw Snake 1
	if gs.Snakes[0].Alive {
		for i, segment := range gs.Snakes[0].Body {
			if segment.Y >= 0 && segment.Y < gs.GridHeight && segment.X >= 0 && segment.X < gs.GridWidth {
				if i == 0 {
					grid[segment.Y][segment.X] = '1' // Head
				} else {
					grid[segment.Y][segment.X] = 'o'
				}
			}
		}
	}

	// Draw Snake 2
	if gs.Snakes[1].Alive {
		for i, segment := range gs.Snakes[1].Body {
			if segment.Y >= 0 && segment.Y < gs.GridHeight && segment.X >= 0 && segment.X < gs.GridWidth {
				if i == 0 {
					grid[segment.Y][segment.X] = '2' // Head
				} else {
					grid[segment.Y][segment.X] = 'x'
				}
			}
		}
	}

	result := fmt.Sprintf("Turn %d\n", gs.Turn)
	for _, row := range grid {
		result += string(row) + "\n"
	}
	result += fmt.Sprintf("Snake 1: Alive=%v, Length=%d, Score=%d, Speed=%d, Sleep=%d, Energy=%d\n", gs.Snakes[0].Alive, gs.Snakes[0].Length, gs.Snakes[0].Score, gs.Snakes[0].SpeedTurns, gs.Snakes[0].SleepTurns, gs.Snakes[0].Energy)
	result += fmt.Sprintf("Snake 2: Alive=%v, Length=%d, Score=%d, Speed=%d, Sleep=%d, Energy=%d\n", gs.Snakes[1].Alive, gs.Snakes[1].Length, gs.Snakes[1].Score, gs.Snakes[1].SpeedTurns, gs.Snakes[1].SleepTurns, gs.Snakes[1].Energy)

	return result
}
