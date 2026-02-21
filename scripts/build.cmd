@echo off
setlocal enabledelayedexpansion

echo =^> Building frontend...
cd cmd\auspex\web
call npm install
if errorlevel 1 exit /b 1
call npm run build
if errorlevel 1 exit /b 1
cd ..\..\..

echo =^> Generating store (sqlc)...
sqlc generate
if errorlevel 1 exit /b 1

echo =^> Building binary...
go build -o auspex.exe .\cmd\auspex\
if errorlevel 1 exit /b 1

echo Done. Run auspex.exe to start.
