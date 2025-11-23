#!/bin/bash

# Скрипт для создания симлинков на сертификаты Let's Encrypt
# Использование: ./link_letsencrypt_cert.sh yourdomain.com

set -e

DOMAIN="${1}"

if [ -z "$DOMAIN" ]; then
    echo "Использование: $0 <domain>"
    echo "Пример: $0 example.com"
    exit 1
fi

# Определяем корень проекта (на уровень выше от ci-cd)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

CERT_PATH="/etc/letsencrypt/live/$DOMAIN/fullchain.pem"
KEY_PATH="/etc/letsencrypt/live/$DOMAIN/privkey.pem"

# Проверка существования сертификатов
if [ ! -f "$CERT_PATH" ]; then
    echo "Ошибка: Сертификат не найден: $CERT_PATH"
    echo "Сначала получите сертификат: ./get_letsencrypt_cert.sh $DOMAIN your@email.com"
    exit 1
fi

if [ ! -f "$KEY_PATH" ]; then
    echo "Ошибка: Приватный ключ не найден: $KEY_PATH"
    exit 1
fi

echo "Создание симлинков на сертификаты Let's Encrypt..."
echo ""

# Переходим в корень проекта
cd "$PROJECT_ROOT"

# Удаление старых файлов, если они существуют
if [ -f "server.crt" ] || [ -L "server.crt" ]; then
    echo "Удаление старого server.crt..."
    rm -f "server.crt"
fi

if [ -f "server.key" ] || [ -L "server.key" ]; then
    echo "Удаление старого server.key..."
    rm -f "server.key"
fi

# Создание симлинков
echo "Создание симлинка: server.crt -> $CERT_PATH"
ln -s "$CERT_PATH" "server.crt"

echo "Создание симлинка: server.key -> $KEY_PATH"
ln -s "$KEY_PATH" "server.key"

echo ""
echo "✓ Симлинки успешно созданы!"
echo ""
echo "Примечание: Для работы симлинков приложение должно иметь права на чтение"
echo "сертификатов Let's Encrypt. Возможно, потребуется добавить пользователя в группу:"
echo "  sudo usermod -a -G ssl-cert \$USER"
echo "  или"
echo "  sudo chmod 755 /etc/letsencrypt/live"
echo "  sudo chmod 755 /etc/letsencrypt/archive"

