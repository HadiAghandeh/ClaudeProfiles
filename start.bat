@echo off
REM Opens the Claude Desktop user menu. Pass -Profile "name" to start one user directly, or -List to list them.
REM Each user gets its own folder under %LOCALAPPDATA%\Claude-Data and is recorded in profiles.json next to this file.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0claude-profiles.ps1" %*
if errorlevel 1 pause
