#!/usr/bin/env python3
"""
Random Snake Bot - Makes random valid moves
"""

import sys
import json
import random

# Force unbuffered output
sys.stdout.reconfigure(line_buffering=True)
sys.stderr.reconfigure(line_buffering=True)


def get_safe_moves(game_state, my_snake):
    """Get all safe moves that don't immediately hit walls"""
    head = my_snake['body'][0]
    all_moves = ["UP", "DOWN", "LEFT", "RIGHT"]
    safe_moves = []
    
    for move in all_moves:
        next_pos = {
            "UP": {"x": head['x'], "y": head['y'] - 1},
            "DOWN": {"x": head['x'], "y": head['y'] + 1},
            "LEFT": {"x": head['x'] - 1, "y": head['y']},
            "RIGHT": {"x": head['x'] + 1, "y": head['y']},
        }[move]
        
        # Check walls
        if (0 <= next_pos['x'] < game_state['grid_width'] and
            0 <= next_pos['y'] < game_state['grid_height']):
            safe_moves.append(move)
    
    return safe_moves if safe_moves else all_moves


def main():
    """Main loop: read game state, pick random move"""
    try:
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue
            
            game_state = json.loads(line)
            my_snake = game_state['snakes'][0]
            
            # Get safe moves and pick one randomly
            safe_moves = get_safe_moves(game_state, my_snake)
            move = random.choice(safe_moves)
            
            response = {"move": move}
            print(json.dumps(response), flush=True)
            
    except Exception as e:
        print(json.dumps({"move": "UP"}), flush=True)
        sys.stderr.write(f"Error: {e}\n")


if __name__ == "__main__":
    main()
