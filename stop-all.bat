@echo off
echo ============================================
echo   QuickSnap - Stop All Services
echo ============================================
echo.

echo Stopping Node.js processes (Frontend)...
taskkill /F /IM node.exe 2>nul
if %errorlevel% equ 0 (
    echo [OK] Node.js processes stopped
) else (
    echo [INFO] No Node.js processes running
)

echo.
echo Stopping Go processes (Backend API & Worker)...
taskkill /F /IM api.exe 2>nul
taskkill /F /IM worker.exe 2>nul
if %errorlevel% equ 0 (
    echo [OK] Go processes stopped
) else (
    echo [INFO] No Go processes running
)

echo.
echo ============================================
echo   All services stopped!
echo ============================================
echo.
pause
