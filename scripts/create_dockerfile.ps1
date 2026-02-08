#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Creates a Dockerfile for a bot directory

.DESCRIPTION
    This script generates a Dockerfile and requirements.txt (if needed) for your bot.
    It helps containerize your bot for Docker submission.

.PARAMETER BotDir
    The bot directory path (e.g., ./bots/your_team_name)

.PARAMETER ImageName
    Optional: The Docker image name (defaults to bot directory name)

.EXAMPLE
    .\scripts\create_dockerfile.ps1 -BotDir ./bots/my_team
    
.EXAMPLE
    .\scripts\create_dockerfile.ps1 -BotDir ./bots/my_team -ImageName my-team-bot
#>

param(
    [Parameter(Mandatory=$true, HelpMessage="Bot directory path")]
    [string]$BotDir,
    
    [Parameter(Mandatory=$false, HelpMessage="Docker image name")]
    [string]$ImageName = ""
)

# Normalize path
$BotDir = $BotDir.TrimEnd('\', '/')
$BotPath = Resolve-Path -Path $BotDir -ErrorAction SilentlyContinue

if (-not $BotPath) {
    Write-Error "Bot directory not found: $BotDir"
    exit 1
}

# Check if config.json exists
$ConfigPath = Join-Path $BotPath "config.json"
if (-not (Test-Path $ConfigPath)) {
    Write-Error "config.json not found in $BotPath"
    exit 1
}

# Read config to get bot name
try {
    $Config = Get-Content $ConfigPath | ConvertFrom-Json
    $BotName = $Config.name
    Write-Host "Found bot: $BotName" -ForegroundColor Green
    
    # Extract Python filename from command array
    $BotFile = "bot.py"  # Default
    if ($Config.command -and $Config.command.Count -gt 0) {
        foreach ($arg in $Config.command) {
            if ($arg -match '\.py$') {
                $BotFile = $arg
                break
            }
        }
    }
    Write-Host "Python file: $BotFile" -ForegroundColor Cyan
} catch {
    Write-Error "Failed to parse config.json: $_"
    exit 1
}

# Determine image name
if ([string]::IsNullOrWhiteSpace($ImageName)) {
    $ImageName = (Split-Path -Leaf $BotPath).ToLower() -replace '_', '-' -replace '[^a-z0-9-]', '' -replace '^-+', '' -replace '-+$', ''
}

Write-Host "`nCreating Docker files for bot in: $BotPath" -ForegroundColor Cyan

# Create Dockerfile
$DockerfilePath = Join-Path $BotPath "Dockerfile"
$DockerfileContent = @"
FROM python:3.12-slim

# Set working directory
WORKDIR /bot

# Copy bot files
COPY . /bot

# Install dependencies if requirements.txt exists
RUN pip install --no-cache-dir -r requirements.txt || true

# Run the bot (unbuffered output)
ENTRYPOINT ["python", "-u", "$BotFile"]
"@

try {
    Set-Content -Path $DockerfilePath -Value $DockerfileContent
    Write-Host "[OK] Created Dockerfile" -ForegroundColor Green
} catch {
    Write-Error "Failed to create Dockerfile: $_"
    exit 1
}

# Create requirements.txt if it doesn't exist
$RequirementsPath = Join-Path $BotPath "requirements.txt"
if (-not (Test-Path $RequirementsPath)) {
    $RequirementsContent = @"
# Add Python dependencies here if needed
# Example:
# numpy==1.26.0
# scipy==1.11.0
# requests==2.31.0
"@
    try {
        Set-Content -Path $RequirementsPath -Value $RequirementsContent
        Write-Host "[OK] Created requirements.txt (empty template)" -ForegroundColor Green
    } catch {
        Write-Warning "Failed to create requirements.txt: $_"
    }
} else {
    Write-Host "[SKIP] requirements.txt already exists" -ForegroundColor Yellow
}

# Create .dockerignore if it doesn't exist
$DockerIgnorePath = Join-Path $BotPath ".dockerignore"
if (-not (Test-Path $DockerIgnorePath)) {
    $DockerIgnoreContent = @"
*.log
__pycache__
*.pyc
.git
.gitignore
README.md
"@
    try {
        Set-Content -Path $DockerIgnorePath -Value $DockerIgnoreContent
        Write-Host "[OK] Created .dockerignore" -ForegroundColor Green
    } catch {
        Write-Warning "Failed to create .dockerignore: $_"
    }
}

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "Docker files created successfully!" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Cyan

# Build the Docker image
Write-Host "`n[Building Docker image...]" -ForegroundColor Cyan
Write-Host "Running: docker build -t $ImageName $BotPath" -ForegroundColor Yellow

try {
    docker build -t $ImageName $BotPath
    if ($LASTEXITCODE -eq 0) {
        Write-Host "`n[SUCCESS] Docker image built successfully: $ImageName" -ForegroundColor Green
    } else {
        Write-Host "`n[ERROR] Docker build failed with exit code: $LASTEXITCODE" -ForegroundColor Red
        Write-Host "Check the output above for errors." -ForegroundColor Yellow
        exit 1
    }
} catch {
    Write-Error "Failed to build Docker image: $_"
    exit 1
}

Write-Host "`n  Update config.json to use Docker:" -ForegroundColor Yellow
Write-Host ""
Write-Host '     {' -ForegroundColor White
Write-Host '       "command": ["python", "-u", "bot.py"],' -ForegroundColor White
Write-Host '       "name": "' -NoNewline -ForegroundColor White
Write-Host $BotName -NoNewline -ForegroundColor Cyan
Write-Host '",' -ForegroundColor White
Write-Host '       "docker_image": "' -NoNewline -ForegroundColor White
Write-Host $ImageName -NoNewline -ForegroundColor Cyan
Write-Host '",' -ForegroundColor White
Write-Host '       "docker_cpus": 0.5,' -ForegroundColor White
Write-Host '       "docker_memory": "256m"' -ForegroundColor White
Write-Host '     }' -ForegroundColor White
Write-Host ""
