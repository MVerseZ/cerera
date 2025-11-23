#!/bin/bash

# Скрипт для получения SSL сертификата от Let's Encrypt
# Использование: ./get_letsencrypt_cert.sh yourdomain.com your@email.com

set -e

DOMAIN="${1}"
EMAIL="${2}"

if [ -z "$DOMAIN" ] || [ -z "$EMAIL" ]; then
    echo "Использование: $0 <domain> <email>"
    echo "Пример: $0 example.com admin@example.com"
    exit 1
fi

# Определяем корень проекта
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Получение SSL сертификата от Let's Encrypt для домена: $DOMAIN"
echo "Email: $EMAIL"
echo ""

# Проверка наличия certbot
if ! command -v certbot &> /dev/null; then
    echo "Ошибка: certbot не установлен."
    echo ""
    echo "Установите certbot:"
    echo "  Ubuntu/Debian: sudo apt-get update && sudo apt-get install certbot"
    echo "  CentOS/RHEL:   sudo yum install certbot"
    echo "  macOS:          brew install certbot"
    exit 1
fi

# Проверка прав root (для certbot нужны права администратора)
if [ "$EUID" -ne 0 ]; then 
    echo "Внимание: certbot требует прав администратора."
    echo "Запустите скрипт с sudo: sudo $0 $DOMAIN $EMAIL"
    exit 1
fi

# Получение сертификата через standalone режим (требует остановки сервера)
echo "Получение сертификата в standalone режиме..."
echo "ВНИМАНИЕ: Убедитесь, что порт 80 свободен и сервер остановлен!"
echo ""
read -p "Продолжить? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    exit 1
fi

# Запуск certbot
certbot certonly --standalone \
    --non-interactive \
    --agree-tos \
    --email "$EMAIL" \
    -d "$DOMAIN" \
    --preferred-challenges http

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Сертификат успешно получен!"
    echo ""
    echo "Сертификаты находятся в:"
    echo "  Cert: /etc/letsencrypt/live/$DOMAIN/fullchain.pem"
    echo "  Key:  /etc/letsencrypt/live/$DOMAIN/privkey.pem"
    echo ""
    echo "Теперь создайте симлинки или скопируйте сертификаты в корень проекта:"
    echo "  cd $PROJECT_ROOT"
    echo "  sudo ln -s /etc/letsencrypt/live/$DOMAIN/fullchain.pem ./server.crt"
    echo "  sudo ln -s /etc/letsencrypt/live/$DOMAIN/privkey.pem ./server.key"
    echo ""
    echo "Или используйте скрипт: cd $PROJECT_ROOT/ci-cd && ./link_letsencrypt_cert.sh $DOMAIN"
    echo ""
    echo "Для автоматического обновления сертификатов добавьте в crontab:"
    echo "  0 0 * * * certbot renew --quiet && systemctl reload your-service"
else
    echo ""
    echo "✗ Ошибка при получении сертификата"
    exit 1
fi

