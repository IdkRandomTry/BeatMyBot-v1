#!/bin/bash
# Quick test script for Snake Game Engine (Unix/Linux/Mac)
# Runs a test match between the two example bots
# Usage: ./scripts/test_match.sh [bot1] [bot2] [map]
# Example: ./scripts/test_match.sh example_python random_bot maps/large.json

# Set default values if arguments not provided
BOT1=${1:-example_python}
BOT2=${2:-example_python}
MAP=${3:-maps/large.json}

echo "========================================"
echo "  Snake Game Engine - Quick Test"
echo "========================================"
echo ""

echo "Building the engine..."
go build -o bin/snakegame
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo ""
echo "Running test match..."
echo "Bot1: $BOT1"
echo "Bot2: $BOT2"
echo "Map: $MAP"
echo ""

./bin/snakegame -bot1 $BOT1 -bot2 $BOT2 -map $MAP -verbose

echo ""
echo "========================================"
echo "Test complete! Check replays/match_replay.json"
echo "========================================"
