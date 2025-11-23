#!/bin/bash

# Скрипт для генерации самоподписанного SSL сертификата для HTTPS
# Использование: ./generate_cert.sh
# Запускайте из корня проекта или из папки ci-cd

# Определяем корень проекта (на уровень выше от ci-cd)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

echo "Генерация самоподписанного SSL сертификата..."

# Проверка наличия openssl
if ! command -v openssl &> /dev/null; then
    echo "Ошибка: openssl не установлен. Установите openssl для продолжения."
    exit 1
fi

# Генерация приватного ключа
echo "Создание приватного ключа (server.key)..."
openssl genrsa -out server.key 2048

# Генерация самоподписанного сертификата
echo "Создание самоподписанного сертификата (server.crt)..."
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365 -subj "/C=RU/ST=State/L=City/O=Organization/CN=localhost"

echo ""
echo "✓ Сертификат успешно создан!"
echo "  - server.key (приватный ключ)"
echo "  - server.crt (сертификат)"
echo ""
echo "Примечание: Это самоподписанный сертификат. Браузеры будут показывать предупреждение о безопасности."
echo "Для продакшена используйте сертификаты от доверенного CA (например, Let's Encrypt)."

