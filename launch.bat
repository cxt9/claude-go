@echo off
REM Claude Code Go - Portable Launcher (Windows)

setlocal enabledelayedexpansion

REM Get the directory where this script is located (USB root)
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

REM Platform is always windows-amd64 for Windows
set "PLATFORM=windows-amd64"

REM Path to the launcher binary
set "LAUNCHER=%SCRIPT_DIR%\bin\%PLATFORM%\claude-go.exe"

REM Check if binary exists
if not exist "%LAUNCHER%" (
    echo Error: Launcher binary not found at %LAUNCHER%
    echo Please ensure Claude Code Go is properly installed.
    exit /b 1
)

REM Run the launcher
"%LAUNCHER%" %*
