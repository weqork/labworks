@echo off
setlocal EnableExtensions
pushd "%~dp0" || exit /b 1
if not exist "Start-Messenger.exe" goto failed
if not exist "Start-PostgreSQL.exe" goto failed

echo Preparing PostgreSQL. The first launch can take a minute or two.
start "" /wait "%~dp0Start-PostgreSQL.exe" --quiet
if errorlevel 1 goto failed

"Start-Messenger.exe" --no-pause
if errorlevel 1 goto failed
popd
exit /b 0

:failed
echo Startup failed. Open Start-PostgreSQL.exe for database diagnostics.
pause
popd
exit /b 1
