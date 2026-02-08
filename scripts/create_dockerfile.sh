#!/bin/bash
# Creates a Dockerfile for a bot directory

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Usage function
usage() {
    echo "Usage: $0 <bot_directory> [image_name]"
    echo ""
    echo "Example:"
    echo "  $0 ./bots/my_team"
    echo "  $0 ./bots/my_team my-team-bot"
    exit 1
}

# Check arguments
if [ $# -lt 1 ]; then
    usage
fi

BOT_DIR="${1%/}"
IMAGE_NAME="${2:-}"

# Check if directory exists
if [ ! -d "$BOT_DIR" ]; then
    echo -e "${RED}Error: Bot directory not found: $BOT_DIR${NC}"
    exit 1
fi

# Check if config.json exists
if [ ! -f "$BOT_DIR/config.json" ]; then
    echo -e "${RED}Error: config.json not found in $BOT_DIR${NC}"
    exit 1
fi

# Read bot name from config
BOT_NAME=$(grep -o '"name"[[:space:]]*:[[:space:]]*"[^"]*"' "$BOT_DIR/config.json" | cut -d'"' -f4)
echo -e "${GREEN}Found bot: $BOT_NAME${NC}"

# Extract Python filename from config.json command array
BOT_FILE="bot.py"  # Default
if command -v python3 &> /dev/null; then
    BOT_FILE=$(python3 -c "
import json
try:
    with open('$BOT_DIR/config.json') as f:
        config = json.load(f)
        for arg in config.get('command', []):
            if arg.endswith('.py'):
                print(arg)
                break
        else:
            print('bot.py')
except:
    print('bot.py')
" 2>/dev/null || echo "bot.py")
else
    # Fallback: try to extract .py file from config using grep
    TEMP_FILE=$(grep -o '"[^"]*\.py"' "$BOT_DIR/config.json" | head -1 | tr -d '"' || echo "bot.py")
    if [ -n "$TEMP_FILE" ]; then
        BOT_FILE="$TEMP_FILE"
    fi
fi
echo -e "${CYAN}Python file: $BOT_FILE${NC}"

# Determine image name
if [ -z "$IMAGE_NAME" ]; then
    IMAGE_NAME=$(basename "$BOT_DIR" | tr '[:upper:]' '[:lower:]' | tr '_' '-' | sed 's/[^a-z0-9-]//g' | sed 's/^-+//;s/-+$//')
fi

echo -e "\n${CYAN}Creating Docker files for bot in: $BOT_DIR${NC}"

# Create Dockerfile (using variable substitution, not heredoc)
cat > "$BOT_DIR/Dockerfile" << EOF
FROM python:3.12-slim

# Set working directory
WORKDIR /bot

# Copy bot files
COPY . /bot

# Install dependencies if requirements.txt exists
RUN pip install --no-cache-dir -r requirements.txt || true

# Run the bot (unbuffered output)
ENTRYPOINT ["python", "-u", "$BOT_FILE"]
EOF

echo -e "${GREEN}✓ Created Dockerfile${NC}"

# Create requirements.txt if it doesn't exist
if [ ! -f "$BOT_DIR/requirements.txt" ]; then
    cat > "$BOT_DIR/requirements.txt" << 'EOF'
# Add Python dependencies here if needed
# Example:
# numpy==1.26.0
# scipy==1.11.0
# requests==2.31.0
EOF
    echo -e "${GREEN}✓ Created requirements.txt (empty template)${NC}"
else
    echo -e "${YELLOW}✓ requirements.txt already exists${NC}"
fi

# Create .dockerignore if it doesn't exist
if [ ! -f "$BOT_DIR/.dockerignore" ]; then
    cat > "$BOT_DIR/.dockerignore" << 'EOF'
*.log
__pycache__
*.pyc
.git
.gitignore
README.md
EOF
    echo -e "${GREEN}✓ Created .dockerignore${NC}"
fi

echo -e "\n${CYAN}========================================${NC}"
echo -e "${GREEN}Docker files created successfully!${NC}"
echo -e "${CYAN}========================================${NC}"

# Build the Docker image
echo -e "\n${CYAN}[Building Docker image...]${NC}"
echo -e "${YELLOW}Running: docker build -t $IMAGE_NAME $BOT_DIR${NC}"

if docker build -t "$IMAGE_NAME" "$BOT_DIR"; then
    echo -e "\n${GREEN}✓ Docker image built successfully: $IMAGE_NAME${NC}"
else
    echo -e "\n${RED}✗ Docker build failed${NC}"
    echo -e "${YELLOW}Check the output above for errors.${NC}"
    exit 1
fi

echo -e "\n${YELLOW}Update config.json to use Docker:${NC}"
echo ""
echo '     {'
echo '       "command": ["python", "-u", "bot.py"],'
echo '       "name": "'"$BOT_NAME"'",'
echo '       "docker_image": "'"$IMAGE_NAME"'",'
echo '       "docker_cpus": 0.5,'
echo '       "docker_memory": "256m"'
echo '     }'
echo ""
