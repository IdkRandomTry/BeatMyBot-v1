package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"snakegame/engine"
)

func main() {
	// Define command-line flags
	bot1Dir := flag.String("bot1", "", "Bot 1 folder name in bots directory (required)")
	bot2Dir := flag.String("bot2", "", "Bot 2 folder name in bots directory (required)")
	width := flag.Int("width", 20, "Grid width")
	height := flag.Int("height", 20, "Grid height")
	maxTurns := flag.Int("max-turns", 500, "Maximum number of turns")
	timeout := flag.Int("timeout", 500, "Turn timeout in milliseconds")
	replayOutput := flag.String("output", "replays/match_replay.json", "Replay output file")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	mapPath := flag.String("map", "", "Optional map JSON file with obstacles")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Snake Game Engine - A competitive Snake game for bot battles\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Required flags:\n")
		fmt.Fprintf(os.Stderr, "  -bot1 string\n")
		fmt.Fprintf(os.Stderr, "        Bot 1 folder name in bots directory (must contain config.json)\n")
		fmt.Fprintf(os.Stderr, "  -bot2 string\n")
		fmt.Fprintf(os.Stderr, "        Bot 2 folder name in bots directory (must contain config.json)\n\n")
		fmt.Fprintf(os.Stderr, "Optional flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -bot1 player1 -bot2 player2 -verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -bot1 python_bot -bot2 go_bot -width 25 -height 25 -max-turns 1000\n\n", os.Args[0])
	}

	flag.Parse()

	// Validate required arguments
	if *bot1Dir == "" || *bot2Dir == "" {
		fmt.Fprintf(os.Stderr, "Error: Both -bot1 and -bot2 flags are required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Prepend .\bots\ to the folder names
	bot1Path := filepath.Join(".", "bots", *bot1Dir)
	bot2Path := filepath.Join(".", "bots", *bot2Dir)

	// Convert to absolute paths
	bot1AbsPath, err := filepath.Abs(bot1Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid bot1 path: %v\n", err)
		os.Exit(1)
	}

	bot2AbsPath, err := filepath.Abs(bot2Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Invalid bot2 path: %v\n", err)
		os.Exit(1)
	}

	// Check if directories exist
	if _, err := os.Stat(bot1AbsPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Bot 1 directory does not exist: %s\n", bot1AbsPath)
		os.Exit(1)
	}

	if _, err := os.Stat(bot2AbsPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Bot 2 directory does not exist: %s\n", bot2AbsPath)
		os.Exit(1)
	}

	// Check for config.json in both directories
	if _, err := os.Stat(filepath.Join(bot1AbsPath, "config.json")); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config.json not found in bot 1 directory: %s\n", bot1AbsPath)
		os.Exit(1)
	}

	if _, err := os.Stat(filepath.Join(bot2AbsPath, "config.json")); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: config.json not found in bot 2 directory: %s\n", bot2AbsPath)
		os.Exit(1)
	}

	// Create match configuration
	config := engine.MatchConfig{
		GridWidth:    *width,
		GridHeight:   *height,
		MaxTurns:     *maxTurns,
		TurnTimeout:  time.Duration(*timeout) * time.Millisecond,
		Bot1Dir:      bot1AbsPath,
		Bot2Dir:      bot2AbsPath,
		ReplayOutput: *replayOutput,
		Verbose:      *verbose,
		MapPath:      *mapPath,
	}

	// Print configuration
	fmt.Println("╔═══════════════════════════════════════════════╗")
	fmt.Println("║       SNAKE GAME ENGINE - MATCH START        ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
	fmt.Printf("\nConfiguration:\n")
	fmt.Printf("  Grid Size:     %dx%d\n", config.GridWidth, config.GridHeight)
	fmt.Printf("  Max Turns:     %d\n", config.MaxTurns)
	fmt.Printf("  Turn Timeout:  %dms\n", *timeout)
	fmt.Printf("  Replay Output: %s\n", config.ReplayOutput)
	fmt.Printf("\nBot 1: %s\n", bot1AbsPath)
	fmt.Printf("Bot 2: %s\n", bot2AbsPath)
	fmt.Println()

	// Create and run the match
	match, err := engine.NewMatch(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating match: %v\n", err)
		os.Exit(1)
	}

	// Handle cleanup on exit
	defer match.Stop()

	// Run the match
	if err := match.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running match: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n╔═══════════════════════════════════════════════╗")
	fmt.Println("║           MATCH COMPLETED SUCCESSFULLY        ║")
	fmt.Println("╚═══════════════════════════════════════════════╝")
}
