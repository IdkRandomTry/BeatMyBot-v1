#!/usr/bin/env python3
"""
Simple Snake Bot Example
Reads game state from stdin (JSON) and outputs move to stdout (JSON)
"""

import sys
import json

# Force unbuffered output
sys.stdout.reconfigure(line_buffering=True)
sys.stderr.reconfigure(line_buffering=True)


def find_nearest_apple(head, apples):
    """Find the nearest apple using Manhattan distance
    
    Each apple is a dict with 'x', 'y', and 'type' fields:
    - 'type' can be: "NORMAL", "GOD", "SPEED", "SLEEP", "POISON"
    
    You can prioritize certain apple types in your strategy!
    """
    if not apples:
        return None
    
    min_dist = float('inf')
    nearest = None
    
    for apple in apples:
        # Calculate distance to apple
        dist = abs(head['x'] - apple['x']) + abs(head['y'] - apple['y'])
        
        # You could add logic here to prioritize certain apple types
        # For example, avoid POISON apples if possible:
        if apple['type'] == 'POISON':
            dist += 10  # Add penalty to avoid poison apples
        
        # Prefer high-value apples (GOD > others)
        if apple['type'] == 'GOD':
            dist -= 5  # Subtract penalty to prefer god apples
        
        if dist < min_dist:
            min_dist = dist
            nearest = apple
    
    return nearest


def get_move_towards(head, target):
    """Determine the best move to get closer to target"""
    dx = target['x'] - head['x']
    dy = target['y'] - head['y']
    
    # Prioritize larger distance
    if abs(dx) > abs(dy):
        return "RIGHT" if dx > 0 else "LEFT"
    else:
        return "DOWN" if dy > 0 else "UP"


def is_safe_move(head, move, grid_width, grid_height, obstacles):
    """Check if a move is safe (doesn't hit walls or obstacles)"""
    next_pos = {
        "UP": {"x": head['x'], "y": head['y'] - 1},
        "DOWN": {"x": head['x'], "y": head['y'] + 1},
        "LEFT": {"x": head['x'] - 1, "y": head['y']},
        "RIGHT": {"x": head['x'] + 1, "y": head['y']},
    }[move]
    
    # Check walls
    if next_pos['x'] < 0 or next_pos['x'] >= grid_width:
        return False
    if next_pos['y'] < 0 or next_pos['y'] >= grid_height:
        return False
    
    # Check obstacles (snake bodies)
    for obs in obstacles:
        if next_pos['x'] == obs['x'] and next_pos['y'] == obs['y']:
            return False
    
    return True


def get_all_obstacles(game_state):
    """Get all obstacle positions (all snake bodies)"""
    obstacles = []
    for snake in game_state['snakes']:
        # Include all body segments
        obstacles.extend(snake['body'])
    # Include static map obstacles if present
    map_data = game_state.get('map')
    if map_data:
        map_obs = map_data.get('obstacles', [])
        obstacles.extend(map_obs)
    return obstacles


def decide_move(game_state):
    """Main decision logic for the bot
    
    SPECIAL APPLES GUIDE:
    - NORMAL: Regular apple, +1 length
    - GOD: Worth 3 points (+3 length)
    - SPEED: Gives you 2 moves per turn for 5 turns
    - SLEEP: Freezes opponent for 5 turns (no moves)
    - POISON: Reduces your length by 1 (avoid!)
    """
    # First snake always ours
    my_snake = game_state['snakes'][0]
    
    # Check if we're alive
    if not my_snake['alive']:
        # return bogus value if dead
        return my_snake['direction']
    
    # Check our current status
    my_length = my_snake['length']
    my_score = my_snake['score']
    speed_active = my_snake['speed_turns'] > 0  # Are we currently moving 2x speed?
    frozen = my_snake['sleep_turns'] > 0  # Are we frozen?
    
    # Check opponent status
    opponent = game_state['snakes'][1]
    opponent_frozen = opponent['sleep_turns'] > 0
    
    head = my_snake['body'][0]
    apples = game_state['apples']
    
    # Get all obstacles
    obstacles = get_all_obstacles(game_state)
    
    # Find nearest apple
    nearest_apple = find_nearest_apple(head, apples)
    
    if nearest_apple:
        # Try to move towards apple
        preferred_move = get_move_towards(head, nearest_apple)
        
        if is_safe_move(head, preferred_move, 
                       game_state['grid_width'], 
                       game_state['grid_height'], 
                       obstacles):
            return preferred_move
    
    # If we can't move towards apple, try any safe move
    all_moves = ["UP", "DOWN", "LEFT", "RIGHT"]
    for move in all_moves:
        if is_safe_move(head, move, 
                       game_state['grid_width'], 
                       game_state['grid_height'], 
                       obstacles):
            return move
    
    # No safe move found, return current direction
    return my_snake['direction']


def main():
    """Main loop: read game state, decide move, output move"""
    try:
        # Read from stdin line by line
        for line in sys.stdin:
            print(f"Received: {line}", file=sys.stderr, flush=True)
            line = line.strip()
            if not line:
                continue
            
            # Parse game state
            game_state = json.loads(line)
            
            # Decide on a move
            move = decide_move(game_state)
            
            # Output move as JSON
            response = {"move": move}
            print(json.dumps(response), flush=True)
            
    except Exception as e:
        # In case of error, output a default move
        print(json.dumps({"move": "UP"}), flush=True)
        sys.stderr.write(f"Error: {e}\n")


if __name__ == "__main__":
    main()
