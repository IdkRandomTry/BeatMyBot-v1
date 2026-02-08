#!/usr/bin/env python3
"""
Pygame Map Builder for Snake Game
Click to place/remove obstacles, save as JSON
"""

import pygame
import json
import os
import sys

# Colors
WHITE = (255, 255, 255)
BLACK = (0, 0, 0)
GRAY = (200, 200, 200)
DARK_GRAY = (100, 100, 100)
GREEN = (0, 200, 0)
RED = (200, 0, 0)
BLUE = (50, 100, 200)
OBSTACLE_COLOR = (60, 60, 60)

class MapBuilder:
    def __init__(self, grid_width=20, grid_height=20, cell_size=10):
        pygame.init()
        
        self.grid_width = grid_width
        self.grid_height = grid_height
        self.cell_size = cell_size
        self.obstacles = set()
        
        # Window setup
        self.panel_width = 200
        self.window_width = grid_width * cell_size + self.panel_width
        self.window_height = grid_height * cell_size + 100
        
        self.screen = pygame.display.set_mode((self.window_width, self.window_height))
        pygame.display.set_caption("Snake Game Map Builder")
        
        self.font = pygame.font.Font(None, 24)
        self.small_font = pygame.font.Font(None, 18)
        
        self.dragging = False
        self.drag_mode = None  # 'add' or 'remove'
        self.filename = "new_map"
        
        # Button areas
        self.buttons = {
            'save': pygame.Rect(grid_width * cell_size + 10, 20, 180, 35),
            'load': pygame.Rect(grid_width * cell_size + 10, 65, 180, 35),
            'clear': pygame.Rect(grid_width * cell_size + 10, 110, 180, 35),
            'mirror_h': pygame.Rect(grid_width * cell_size + 10, 155, 85, 35),
            'mirror_v': pygame.Rect(grid_width * cell_size + 105, 155, 85, 35),
        }
    
    def get_cell_from_mouse(self, pos):
        """Convert mouse position to grid coordinates"""
        x, y = pos
        if x < 0 or y < 0 or x >= self.grid_width * self.cell_size or y >= self.grid_height * self.cell_size:
            return None
        return (x // self.cell_size, y // self.cell_size)
    
    def toggle_obstacle(self, grid_x, grid_y):
        """Toggle obstacle at grid position"""
        pos = (grid_x, grid_y)
        if pos in self.obstacles:
            self.obstacles.remove(pos)
            return 'remove'
        else:
            self.obstacles.add(pos)
            return 'add'
    
    def add_obstacle(self, grid_x, grid_y):
        """Add obstacle at grid position"""
        if 0 <= grid_x < self.grid_width and 0 <= grid_y < self.grid_height:
            self.obstacles.add((grid_x, grid_y))
    
    def remove_obstacle(self, grid_x, grid_y):
        """Remove obstacle at grid position"""
        pos = (grid_x, grid_y)
        if pos in self.obstacles:
            self.obstacles.discard(pos)
    
    def clear_all(self):
        """Clear all obstacles"""
        self.obstacles.clear()
    
    def mirror_horizontal(self):
        """Mirror obstacles horizontally"""
        new_obstacles = set()
        for x, y in self.obstacles:
            new_obstacles.add((self.grid_width - 1 - x, y))
        self.obstacles.update(new_obstacles)
    
    def mirror_vertical(self):
        """Mirror obstacles vertically"""
        new_obstacles = set()
        for x, y in self.obstacles:
            new_obstacles.add((x, self.grid_height - 1 - y))
        self.obstacles.update(new_obstacles)
    
    def save_map(self, filename):
        """Save map to JSON file"""
        os.makedirs("maps", exist_ok=True)
        filepath = os.path.join("maps", filename)
        if not filepath.endswith('.json'):
            filepath += '.json'
        
        obstacle_list = [{"x": x, "y": y} for x, y in sorted(self.obstacles)]
        data = {
            "width": self.grid_width,
            "height": self.grid_height,
            "obstacles": obstacle_list
        }
        
        with open(filepath, 'w') as f:
            json.dump(data, f, indent=2)
        
        print(f"Saved map to: {filepath}")
        return filepath
    
    def load_map(self, filename):
        """Load map from JSON file"""
        filepath = os.path.join("maps", filename)
        if not filepath.endswith('.json'):
            filepath += '.json'
        
        try:
            with open(filepath, 'r') as f:
                data = json.load(f)
                
                # Load grid dimensions if available
                if 'width' in data and 'height' in data:
                    self.grid_width = data['width']
                    self.grid_height = data['height']
                    # Update window size
                    self.window_width = self.grid_width * self.cell_size + self.panel_width
                    self.window_height = self.grid_height * self.cell_size + 100
                    self.screen = pygame.display.set_mode((self.window_width, self.window_height))
                
                self.obstacles.clear()
                for obs in data.get('obstacles', []):
                    self.add_obstacle(obs['x'], obs['y'])
            print(f"Loaded map from: {filepath}")
            return True
        except FileNotFoundError:
            print(f"File not found: {filepath}")
            return False
        except Exception as e:
            print(f"Error loading map: {e}")
            return False
    
    def draw_grid(self):
        """Draw the grid and obstacles"""
        # Draw grid cells
        for y in range(self.grid_height):
            for x in range(self.grid_width):
                rect = pygame.Rect(x * self.cell_size, y * self.cell_size, 
                                  self.cell_size, self.cell_size)
                
                # Draw cell
                if (x, y) in self.obstacles:
                    pygame.draw.rect(self.screen, OBSTACLE_COLOR, rect)
                else:
                    pygame.draw.rect(self.screen, WHITE, rect)
                
                # Draw grid lines
                pygame.draw.rect(self.screen, GRAY, rect, 1)
    
    def draw_button(self, name, rect, text, color=BLUE):
        """Draw a button"""
        pygame.draw.rect(self.screen, color, rect)
        pygame.draw.rect(self.screen, BLACK, rect, 2)
        
        text_surface = self.small_font.render(text, True, WHITE)
        text_rect = text_surface.get_rect(center=rect.center)
        self.screen.blit(text_surface, text_rect)
    
    def draw_ui(self):
        """Draw UI panel"""
        # Draw panel background
        panel_rect = pygame.Rect(self.grid_width * self.cell_size, 0, 
                                self.panel_width, self.window_height)
        pygame.draw.rect(self.screen, DARK_GRAY, panel_rect)
        
        # Draw buttons
        self.draw_button('save', self.buttons['save'], 'Save', GREEN)
        self.draw_button('load', self.buttons['load'], 'Load', BLUE)
        self.draw_button('clear', self.buttons['clear'], 'Clear All', RED)
        self.draw_button('mirror_h', self.buttons['mirror_h'], 'Mirror H', BLUE)
        self.draw_button('mirror_v', self.buttons['mirror_v'], 'Mirror V', BLUE)
        
        # Draw obstacle count
        count_text = f"Obstacles: {len(self.obstacles)}"
        text_surface = self.small_font.render(count_text, True, WHITE)
        self.screen.blit(text_surface, (self.grid_width * self.cell_size + 10, 210))
        
        # Draw instructions at bottom
        instructions = [
            "Left Click: Add/Remove",
            "Drag: Paint/Erase",
            "ESC: Quit"
        ]
        y_offset = self.window_height - 90
        for instruction in instructions:
            text_surface = self.small_font.render(instruction, True, WHITE)
            self.screen.blit(text_surface, (self.grid_width * self.cell_size + 10, y_offset))
            y_offset += 25
    
    def handle_button_click(self, pos):
        """Handle button clicks"""
        for name, rect in self.buttons.items():
            if rect.collidepoint(pos):
                if name == 'save':
                    filename = input("Enter filename (without .json): ").strip()
                    if filename:
                        self.filename = filename
                        self.save_map(self.filename)
                
                elif name == 'load':
                    filename = input("Enter filename to load (without .json): ").strip()
                    if filename:
                        self.load_map(filename)
                
                elif name == 'clear':
                    self.clear_all()
                
                elif name == 'mirror_h':
                    self.mirror_horizontal()
                
                elif name == 'mirror_v':
                    self.mirror_vertical()
                
                return True
        return False
    
    def run(self):
        """Main game loop"""
        clock = pygame.time.Clock()
        running = True
        
        print("\n=== Snake Game Map Builder ===")
        print("Click to add/remove obstacles")
        print("Drag to paint multiple cells")
        print("Use buttons to save, load, mirror, or clear")
        print("Press ESC to quit\n")
        
        while running:
            for event in pygame.event.get():
                if event.type == pygame.QUIT:
                    running = False
                
                elif event.type == pygame.KEYDOWN:
                    if event.key == pygame.K_ESCAPE:
                        running = False
                    elif event.key == pygame.K_s and pygame.key.get_mods() & pygame.KMOD_CTRL:
                        filename = input("Enter filename (without .json): ").strip() or self.filename
                        self.save_map(filename)
                
                elif event.type == pygame.MOUSEBUTTONDOWN:
                    if event.button == 1:  # Left click
                        # Check if clicking on button
                        if not self.handle_button_click(event.pos):
                            # Click on grid
                            cell = self.get_cell_from_mouse(event.pos)
                            if cell:
                                self.dragging = True
                                self.drag_mode = self.toggle_obstacle(cell[0], cell[1])
                
                elif event.type == pygame.MOUSEBUTTONUP:
                    if event.button == 1:
                        self.dragging = False
                        self.drag_mode = None
                
                elif event.type == pygame.MOUSEMOTION:
                    if self.dragging:
                        cell = self.get_cell_from_mouse(event.pos)
                        if cell:
                            if self.drag_mode == 'add':
                                self.add_obstacle(cell[0], cell[1])
                            elif self.drag_mode == 'remove':
                                self.remove_obstacle(cell[0], cell[1])
            
            # Draw everything
            self.screen.fill(BLACK)
            self.draw_grid()
            self.draw_ui()
            
            pygame.display.flip()
            clock.tick(60)
        
        pygame.quit()
        
        # Ask to save before exit
        if self.obstacles:
            save = input("\nSave map before exit? (y/n): ").strip().lower()
            if save == 'y':
                filename = input("Enter filename (without .json): ").strip() or self.filename
                self.save_map(filename)

def main():
    if len(sys.argv) > 1:
        try:
            width = int(sys.argv[1])
            height = int(sys.argv[2]) if len(sys.argv) > 2 else width
        except:
            width, height = 20, 20
    else:
        width, height = 20, 20
    
    builder = MapBuilder(width, height)
    builder.run()

if __name__ == "__main__":
    main()
