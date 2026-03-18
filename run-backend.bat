@echo off
set PATH=C:\Program Files\Go\bin;%PATH%
cd /d %~dp0apps\backend
go run ./cmd/api
