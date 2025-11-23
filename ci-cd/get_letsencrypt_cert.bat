@echo off
REM Скрипт для получения SSL сертификата от Let's Encrypt (Windows)
REM Использование: get_letsencrypt_cert.bat yourdomain.com your@email.com

setlocal

set DOMAIN=%1
set EMAIL=%2

if "%DOMAIN%"=="" (
    echo Использование: %0 ^<domain^> ^<email^>
    echo Пример: %0 example.com admin@example.com
    exit /b 1
)

if "%EMAIL%"=="" (
    echo Ошибка: не указан email
    echo Использование: %0 ^<domain^> ^<email^>
    exit /b 1
)

REM Определяем корень проекта
cd /d "%~dp0\.."
set PROJECT_ROOT=%CD%

echo Получение SSL сертификата от Let's Encrypt для домена: %DOMAIN%
echo Email: %EMAIL%
echo.

REM Проверка наличия certbot
where certbot >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo Ошибка: certbot не установлен.
    echo.
    echo Установите certbot:
    echo   1. Установите Python 3.x
    echo   2. Установите certbot: pip install certbot
    echo   3. Или используйте WSL (Windows Subsystem for Linux)
    echo.
    echo Рекомендация: Для получения сертификатов Let's Encrypt на Windows
    echo лучше использовать WSL или Linux-сервер.
    echo.
    pause
    exit /b 1
)

echo ВНИМАНИЕ: На Windows получение сертификатов Let's Encrypt может быть сложным.
echo Рекомендуется использовать один из вариантов:
echo.
echo 1. Использовать WSL (Windows Subsystem for Linux)
echo 2. Получить сертификаты на Linux-сервере и скопировать их
echo 3. Использовать certbot в Docker контейнере
echo.
echo Для получения сертификата в standalone режиме:
echo   certbot certonly --standalone --non-interactive --agree-tos -m %EMAIL% -d %DOMAIN%
echo.
echo После получения сертификатов скопируйте их в корень проекта (%PROJECT_ROOT%):
echo   - /etc/letsencrypt/live/%DOMAIN%/fullchain.pem -^> server.crt
echo   - /etc/letsencrypt/live/%DOMAIN%/privkey.pem -^> server.key
echo.
pause

