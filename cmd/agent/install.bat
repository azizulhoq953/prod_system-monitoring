@echo off
echo ========================================
echo Monitor Agent Installer
echo ========================================
echo.

REM Check for admin privileges
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo This installer requires Administrator privileges.
    echo Please right-click and select "Run as Administrator"
    pause
    exit /b 1
)

echo Installing Monitor Agent Service...
echo.

REM Install and start the service
agent.exe install

if %errorLevel% equ 0 (
    echo.
    echo ========================================
    echo Installation Successful!
    echo ========================================
    echo The Monitor Agent is now running in the background.
    echo It will automatically start on system boot.
    echo.
    echo To uninstall, run: agent.exe uninstall
    echo.
) else (
    echo.
    echo Installation failed. Please check the error message above.
    echo.
)

pause