@echo off
REM Claude Code Go - Update Script (Windows)

setlocal enabledelayedexpansion

REM Get the directory where this script is located (USB root)
set "SCRIPT_DIR=%~dp0"
set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

set "PLATFORM=windows-amd64"
set "REPO=cxt9/claude-go"

echo.
echo ===============================================
echo        Claude Code Go - Update Check
echo ===============================================
echo.

REM Read current version
set "CURRENT_VERSION=0.0.0"
if exist "%SCRIPT_DIR%\.version" (
    for /f "tokens=2 delims=:," %%a in ('type "%SCRIPT_DIR%\.version" ^| findstr "version"') do (
        set "CURRENT_VERSION=%%~a"
        set "CURRENT_VERSION=!CURRENT_VERSION:"=!"
    )
)

echo Current version: %CURRENT_VERSION%
echo.

REM Handle offline update
if "%1"=="--offline" (
    if not "%2"=="" (
        echo Applying offline update from %2...

        if not exist "%2" (
            echo Error: File not found: %2
            exit /b 1
        )

        REM Create backup
        echo Creating backup...
        if exist "%SCRIPT_DIR%\.rollback" rd /s /q "%SCRIPT_DIR%\.rollback"
        xcopy /e /i /q "%SCRIPT_DIR%\bin" "%SCRIPT_DIR%\.rollback\bin" >nul

        REM Extract update (requires PowerShell)
        echo Extracting update...
        powershell -Command "Expand-Archive -Path '%2' -DestinationPath '%SCRIPT_DIR%' -Force"

        REM Cleanup
        if exist "%SCRIPT_DIR%\.rollback" rd /s /q "%SCRIPT_DIR%\.rollback"
        if exist "%SCRIPT_DIR%\cache" rd /s /q "%SCRIPT_DIR%\cache"
        mkdir "%SCRIPT_DIR%\cache"

        echo.
        echo Update applied successfully!
        exit /b 0
    )
)

echo Checking for updates...
echo.

REM Download manifest using PowerShell
set "MANIFEST_URL=https://github.com/%REPO%/releases/latest/download/manifest.json"
set "TEMP_MANIFEST=%TEMP%\claude-go-manifest.json"

powershell -Command "(New-Object Net.WebClient).DownloadFile('%MANIFEST_URL%', '%TEMP_MANIFEST%')" 2>nul

if not exist "%TEMP_MANIFEST%" (
    echo Error: Could not check for updates. No internet connection?
    exit /b 1
)

REM Parse version from manifest
for /f "tokens=2 delims=:," %%a in ('type "%TEMP_MANIFEST%" ^| findstr "version"') do (
    set "LATEST_VERSION=%%~a"
    set "LATEST_VERSION=!LATEST_VERSION:"=!"
    goto :version_found
)
:version_found

if "%LATEST_VERSION%"=="" (
    echo Error: Could not parse version from manifest
    del "%TEMP_MANIFEST%" 2>nul
    exit /b 1
)

del "%TEMP_MANIFEST%" 2>nul

REM Compare versions (simple string comparison - works for semantic versioning)
if "%LATEST_VERSION%" gtr "%CURRENT_VERSION%" (
    echo New version available: %LATEST_VERSION%
    echo.

    set /p "CONFIRM=Proceed with update? [Y/n] "
    if /i not "!CONFIRM!"=="n" (
        REM Create backup
        echo Backing up current version...
        if exist "%SCRIPT_DIR%\.rollback" rd /s /q "%SCRIPT_DIR%\.rollback"
        xcopy /e /i /q "%SCRIPT_DIR%\bin" "%SCRIPT_DIR%\.rollback\bin" >nul

        REM Download update
        set "DOWNLOAD_URL=https://github.com/%REPO%/releases/download/v%LATEST_VERSION%/claude-go-%LATEST_VERSION%-%PLATFORM%.zip"
        set "TEMP_ZIP=%TEMP%\claude-go-update.zip"

        echo Downloading update...
        powershell -Command "(New-Object Net.WebClient).DownloadFile('!DOWNLOAD_URL!', '!TEMP_ZIP!')"

        if exist "!TEMP_ZIP!" (
            echo Installing update...
            powershell -Command "Expand-Archive -Path '!TEMP_ZIP!' -DestinationPath '%SCRIPT_DIR%' -Force"

            REM Update version file
            echo {"version":"%LATEST_VERSION%","updated_at":"%DATE%T%TIME%"} > "%SCRIPT_DIR%\.version"

            REM Cleanup
            del "!TEMP_ZIP!" 2>nul
            if exist "%SCRIPT_DIR%\.rollback" rd /s /q "%SCRIPT_DIR%\.rollback"
            if exist "%SCRIPT_DIR%\cache" rd /s /q "%SCRIPT_DIR%\cache"
            mkdir "%SCRIPT_DIR%\cache"

            echo.
            echo Update complete! Now on version %LATEST_VERSION%
            echo Your credentials, sessions, and settings were preserved.
        ) else (
            echo Download failed. Restoring backup...
            rd /s /q "%SCRIPT_DIR%\bin"
            move "%SCRIPT_DIR%\.rollback\bin" "%SCRIPT_DIR%\bin" >nul
            rd /s /q "%SCRIPT_DIR%\.rollback"
            exit /b 1
        )
    ) else (
        echo Update cancelled.
    )
) else (
    echo Already up to date ^(%CURRENT_VERSION%^)
)

echo.
