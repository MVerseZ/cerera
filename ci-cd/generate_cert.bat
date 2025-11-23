@echo off
REM Скрипт для генерации самоподписанного SSL сертификата для HTTPS
REM Использование: generate_cert.bat
REM Запускайте из корня проекта или из папки ci-cd

setlocal

REM Определяем корень проекта (на уровень выше от ci-cd)
cd /d "%~dp0\.."

echo Генерация самоподписанного SSL сертификата...
echo.

REM Проверка наличия openssl
where openssl >nul 2>&1
if %ERRORLEVEL% NEQ 0 (
    echo Ошибка: openssl не установлен.
    echo.
    echo Установите openssl одним из способов:
    echo 1. Через Chocolatey: choco install openssl
    echo 2. Через Git for Windows (входит в комплект)
    echo 3. Скачайте с https://slproweb.com/products/Win32OpenSSL.html
    echo.
    pause
    exit /b 1
)

REM Генерация приватного ключа
echo Создание приватного ключа (server.key)...
openssl genrsa -out server.key 2048
if %ERRORLEVEL% NEQ 0 (
    echo Ошибка при создании ключа
    pause
    exit /b 1
)

REM Генерация самоподписанного сертификата
echo Создание самоподписанного сертификата (server.crt)...
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365 -subj "/C=RU/ST=State/L=City/O=Organization/CN=localhost"
if %ERRORLEVEL% NEQ 0 (
    echo Ошибка при создании сертификата
    pause
    exit /b 1
)

echo.
echo ✓ Сертификат успешно создан!
echo   - server.key (приватный ключ)
echo   - server.crt (сертификат)
echo.
echo Примечание: Это самоподписанный сертификат. Браузеры будут показывать предупреждение о безопасности.
echo Для продакшена используйте сертификаты от доверенного CA (например, Let's Encrypt).
echo.
pause

