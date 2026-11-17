@echo off
REM Opens the Claude user menu. Pass -Profile "name" to start that user's Claude Desktop directly,
REM add -Code (and optionally -Dir "folder") to start their Claude Code instead, or -List to list them.
REM Each user gets its own folders under %LOCALAPPDATA%\Claude-Data and %LOCALAPPDATA%\Claude-Code-Data,
REM and is recorded in profiles.json next to this file.
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0claude-profiles.ps1" %*
if errorlevel 1 pause
