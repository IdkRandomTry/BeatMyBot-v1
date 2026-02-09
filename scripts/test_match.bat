@echo off
REM Quick test script for Snake Game Engine
REM Runs a test match between the two example bots
REM Usage: scripts\test_match.bat [bot1] [bot2] [map]
REM Example: scripts\test_match.bat example_python random_bot maps/large.json

REM Set default values if arguments not provided
set BOT1=%~1
if "%BOT1%"=="" set BOT1=example_python

set BOT2=%~2
if "%BOT2%"=="" set BOT2=example_python

set MAP=%~3
if "%MAP%"=="" set MAP=maps/large.json

echo ========================================
echo   Snake Game Engine - Quick Test
echo ========================================
echo.

echo Building the engine...
go build -o bin/snakegame.exe
if %ERRORLEVEL% NEQ 0 (
    echo Build failed!
    exit /b 1
)

echo.
echo Running test match...
echo Bot1: %BOT1%
echo Bot2: %BOT2%
echo Map: %MAP%
echo.

.\bin\snakegame.exe -bot1 %BOT1% -bot2 %BOT2% -map %MAP% -verbose

echo.
echo ========================================
echo Test complete! Check replays/match_replay.json
echo ========================================
