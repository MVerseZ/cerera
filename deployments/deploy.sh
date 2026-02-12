#!/bin/bash

# Скрипт для деплоя Cerera на удаленный сервер
# Использование: ./deploy.sh user@server-ip

set -e

if [ -z "$1" ]; then
    echo "Использование: $0 user@server-ip"
    echo "Пример: $0 ubuntu@192.168.1.100"
    exit 1
fi

SERVER=$1
REMOTE_USER=$(echo $SERVER | cut -d@ -f1)
REMOTE_HOST=$(echo $SERVER | cut -d@ -f2)

echo "🚀 Деплой Cerera на $SERVER"
echo "================================"

# Проверка подключения
echo "📡 Проверка подключения к серверу..."
ssh -o ConnectTimeout=5 $SERVER "echo 'Подключение успешно'" || {
    echo "❌ Ошибка: Не удалось подключиться к серверу"
    exit 1
}

# Создание директорий на сервере
echo "📁 Создание директорий на сервере..."
ssh $SERVER "mkdir -p ~/cerera-deploy/cerera"
ssh $SERVER "mkdir -p ~/cerera-data"
ssh $SERVER "mkdir -p ~/cerera-keys"

# Передача файлов (используя rsync или scp)
echo "📦 Копирование файлов на сервер..."
rsync -avz --exclude='.git' --exclude='*.exe' --exclude='build' \
    ./ $SERVER:~/cerera-deploy/cerera/ || {
    echo "⚠️  rsync не найден, используем scp..."
    scp -r ./* $SERVER:~/cerera-deploy/cerera/
}

# Проверка Go на сервере
echo "🔍 Проверка установки Go..."
ssh $SERVER "go version" || {
    echo "❌ Go не установлен на сервере"
    echo "Установите Go вручную или используйте:"
    echo "  wget https://go.dev/dl/go1.23.6.linux-amd64.tar.gz"
    echo "  sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz"
    exit 1
}

# Сборка проекта на сервере
echo "🔨 Сборка проекта на сервере..."
ssh $SERVER "cd ~/cerera-deploy/cerera && go mod download"
ssh $SERVER "cd ~/cerera-deploy/cerera && go build -o cerera ./cmd/cerera"

# Копирование конфигурации
echo "⚙️  Копирование конфигурации..."
if [ -f "config.json" ]; then
    scp config.json $SERVER:~/cerera-data/config.json
fi

# Создание systemd service файла
echo "📝 Создание systemd service..."
ssh $SERVER "cat > /tmp/cerera.service << 'EOFSERVICE'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=$REMOTE_USER
WorkingDirectory=/home/$REMOTE_USER/cerera-deploy/cerera
ExecStart=/home/$REMOTE_USER/cerera-deploy/cerera/cerera \\
    -mode=p2p \\
    -addr=31000 \\
    -http=8080 \\
    -miner=true \\
    -mem=false
Restart=always
RestartSec=10
StandardOutput=append:/home/$REMOTE_USER/cerera-data/cerera.log
StandardError=append:/home/$REMOTE_USER/cerera-data/cerera-error.log

[Install]
WantedBy=multi-user.target
EOFSERVICE
"

# Установка systemd service (требует sudo)
echo "🔧 Установка systemd service..."
echo "⚠️  Требуется ввод пароля sudo на сервере"
ssh -t $SERVER "sudo cp /tmp/cerera.service /etc/systemd/system/cerera.service"
ssh -t $SERVER "sudo systemctl daemon-reload"
ssh -t $SERVER "sudo systemctl enable cerera"

# Запуск сервиса
echo "▶️  Запуск сервиса Cerera..."
read -p "Запустить сервис сейчас? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    ssh -t $SERVER "sudo systemctl start cerera"
    echo "✅ Сервис запущен"
    echo "📊 Проверка статуса..."
    ssh -t $SERVER "sudo systemctl status cerera"
else
    echo "⚠️  Сервис создан, но не запущен. Запустите вручную:"
    echo "   ssh $SERVER 'sudo systemctl start cerera'"
fi

echo ""
echo "✅ Деплой завершен!"
echo "================================"
echo "Полезные команды:"
echo "  Просмотр статуса:  ssh $SERVER 'sudo systemctl status cerera'"
echo "  Просмотр логов:    ssh $SERVER 'journalctl -u cerera -f'"
echo "  Перезапуск:        ssh $SERVER 'sudo systemctl restart cerera'"
echo "  Проверка API:      curl http://$REMOTE_HOST:8080/status"

