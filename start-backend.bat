@echo off
echo ============================================
echo   QuickSnap - Backend API
echo ============================================
echo.
cd /d "%~dp0apps\backend"
set PATH=C:\Program Files\Go\bin;%PATH%
go run ./cmd/api
