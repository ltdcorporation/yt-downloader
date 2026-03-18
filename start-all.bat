@echo off
setlocal enabledelayedexpansion

echo ============================================
echo   QuickSnap - Start All Services
echo ============================================
echo.

:: Check if running in separate window
if "%1"=="backend" (
    goto :run_backend
)
if "%1"=="worker" (
    goto :run_worker
)
if "%1"=="frontend" (
    goto :run_frontend
)

:: Main: Start all services in separate windows
echo Starting all services...
echo.

:: Start Backend API
start "QuickSnap - Backend API" cmd.exe /k "%~f0" backend

:: Wait a bit for backend to initialize
timeout /t 2 /nobreak >nul

:: Start Worker
start "QuickSnap - Worker" cmd.exe /k "%~f0" worker

:: Wait a bit for worker to initialize
timeout /t 2 /nobreak >nul

:: Start Frontend
start "QuickSnap - Frontend" cmd.exe /k "%~f0" frontend

echo.
echo ============================================
echo   All services started!
echo   - Backend API:  http://127.0.0.1:8080
echo   - Frontend:     http://localhost:3000
echo ============================================
echo.
echo Press any key to exit this window...
pause >nul
exit /b 0

:run_backend
echo [Backend API] Starting...
cd /d "%~dp0apps\backend"
set PATH=C:\Program Files\Go\bin;%PATH%
go run ./cmd/api
exit /b

:run_worker
echo [Worker] Starting...
cd /d "%~dp0apps\backend"
set PATH=C:\Program Files\Go\bin;%PATH%
go run ./cmd/worker
exit /b

:run_frontend
echo [Frontend] Starting...
cd /d "%~dp0apps\web"
npm run dev
exit /b
