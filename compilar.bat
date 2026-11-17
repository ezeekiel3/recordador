@echo off
echo ==========================================
echo    Compilando Recordador y Notas en Go...
echo ==========================================
go build -ldflags="-H windowsgui" -o Recordador.exe .
if %errorlevel% neq 0 (
    echo [ERROR] Hubo un problema al compilar.
    pause
    exit /b %errorlevel%
)
echo.
echo [EXITO] Compilacion completada con exito.
echo Se ha generado el archivo 'Recordador.exe'.
echo Puedes ejecutarlo haciendo doble clic sobre el.
pause
