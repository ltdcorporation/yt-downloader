@echo off
echo ============================================
echo   QuickSnap - Worker
echo ============================================
echo.
cd /d "%~dp0apps\backend"
set PATH=C:\Program Files\Go\bin;%PATH%
go run ./cmd/worker
