# Руководство по деплою Cerera на удаленный сервер

Это руководство описывает процесс деплоя проекта Cerera на удаленный Linux-сервер через SSH.

## 📋 Требования

### На локальной машине:
- SSH-клиент
- Git (или другой способ передачи файлов)
- Go 1.23.0+ (для сборки на локальной машине, опционально)

### На сервере:
- Linux (Ubuntu/Debian/CentOS)
- Go 1.23.0 или выше
- Git
- Firewall с открытыми портами:
  - **8080** (HTTP API, по умолчанию)
  - **31000** (P2P сеть)
  - **1337** (если используете этот порт для HTTP)

---

## 🚀 Вариант 1: Деплой через Git (Рекомендуется)

### Шаг 1: Подключение к серверу

```bash
ssh user@your-server-ip
```

### Шаг 2: Установка зависимостей на сервере

```bash
# Обновление пакетов
sudo apt update  # для Ubuntu/Debian
# или
sudo yum update  # для CentOS/RHEL

# Установка Go (если еще не установлен)
wget https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Проверка установки
go version

# Установка необходимых утилит
sudo apt install -y git build-essential  # Ubuntu/Debian
# или
sudo yum install -y git gcc make        # CentOS/RHEL
```

### Шаг 3: Клонирование репозитория

```bash
# Создайте директорию для проекта
mkdir -p ~/cerera-deploy
cd ~/cerera-deploy

# Клонируйте репозиторий (или скопируйте файлы другим способом)
git clone https://github.com/cerera/cerera.git
cd cerera
```

**Или если репозиторий приватный:**
```bash
# Используйте ваш URL репозитория
git clone git@github.com:your-username/cerera.git
```

### Шаг 4: Сборка проекта

```bash
# Скачивание зависимостей
go mod download

# Сборка бинарного файла
go build -o cerera ./cmd/cerera

# Проверка, что файл создан
ls -lh cerera
```

### Шаг 5: Настройка конфигурации

```bash
# Создайте директорию для данных
mkdir -p ~/cerera-data
mkdir -p ~/cerera-keys

# Скопируйте или создайте config.json
cp config.json ~/cerera-data/config.json

# Если нужен ключ для ноды, скопируйте его
# cp ddddd.nodekey.pem ~/cerera-keys/nodekey.pem
```

### Шаг 6: Тестовый запуск

```bash
# Запустите ноду вручную для проверки
./cerera -mode=p2p -addr=31000 -http=8080 -miner=true -mem=false

# Если все работает, остановите процесс (Ctrl+C)
```

---

## 🔧 Вариант 2: Деплой через systemd (Автозапуск)

### Создание systemd service

```bash
# Создайте файл сервиса
sudo nano /etc/systemd/system/cerera.service
```

Вставьте следующую конфигурацию:

```ini
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=YOUR_USERNAME
WorkingDirectory=/home/YOUR_USERNAME/cerera-deploy/cerera
ExecStart=/home/YOUR_USERNAME/cerera-deploy/cerera/cerera \
    -mode=p2p \
    -addr=31000 \
    -http=8080 \
    -miner=true \
    -mem=false \
    -key=/home/YOUR_USERNAME/cerera-keys/nodekey.pem
Restart=always
RestartSec=10
StandardOutput=append:/home/YOUR_USERNAME/cerera-data/cerera.log
StandardError=append:/home/YOUR_USERNAME/cerera-data/cerera-error.log

[Install]
WantedBy=multi-user.target
```

**Замените:**
- `YOUR_USERNAME` на ваше имя пользователя на сервере
- Параметры командной строки по необходимости

### Запуск сервиса

```bash
# Перезагрузка systemd
sudo systemctl daemon-reload

# Включение автозапуска
sudo systemctl enable cerera

# Запуск сервиса
sudo systemctl start cerera

# Проверка статуса
sudo systemctl status cerera

# Просмотр логов
journalctl -u cerera -f
# или
tail -f ~/cerera-data/cerera.log
```

### Управление сервисом

```bash
# Остановка
sudo systemctl stop cerera

# Перезапуск
sudo systemctl restart cerera

# Просмотр статуса
sudo systemctl status cerera

# Отключение автозапуска
sudo systemctl disable cerera
```

---

## 🐳 Вариант 3: Деплой через Docker

### Шаг 1: Установка Docker на сервере

```bash
# Ubuntu/Debian
sudo apt install -y docker.io docker-compose
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker $USER

# Выйдите и зайдите снова для применения изменений группы
```

### Шаг 2: Подготовка файлов для Docker

Убедитесь, что на сервере есть:
- `Dockerfile`
- `ddddd.nodekey.pem` (или ваш файл ключа)
- `docker-compose.yml` (если используете)

### Шаг 3: Сборка Docker образа

```bash
cd ~/cerera-deploy/cerera

# Сборка образа
docker build -t cerera:latest .

# Проверка образа
docker images | grep cerera
```

### Шаг 4: Запуск контейнера

```bash
# Простой запуск
docker run -d \
  --name cerera-node \
  -p 8080:8080 \
  -p 31000:31000 \
  -v ~/cerera-data:/app/data \
  --restart unless-stopped \
  cerera:latest

# Или через docker-compose (если настроен)
docker-compose up -d
```

### Шаг 5: Управление контейнером

```bash
# Просмотр логов
docker logs -f cerera-node

# Остановка
docker stop cerera-node

# Запуск
docker start cerera-node

# Перезапуск
docker restart cerera-node

# Удаление
docker rm -f cerera-node
```

---

## 🔒 Настройка Firewall

```bash
# Ubuntu/Debian (UFW)
sudo ufw allow 8080/tcp
sudo ufw allow 31000/tcp
sudo ufw allow 31000/udp  # для P2P
sudo ufw enable

# CentOS/RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=31000/tcp
sudo firewall-cmd --permanent --add-port=31000/udp
sudo firewall-cmd --reload
```

---

## 📊 Мониторинг (Опционально)

### Prometheus и Grafana

Если нужно развернуть мониторинг:

```bash
cd ~/cerera-deploy/cerera

# Запуск через docker-compose
docker-compose up -d

# Проверка статуса
docker-compose ps

# Доступ к Grafana: http://your-server-ip:3100
# Логин: admin / admin
```

---

## 🔄 Обновление проекта

### При обновлении кода:

```bash
cd ~/cerera-deploy/cerera

# Если используете Git
git pull origin main

# Пересборка
go build -o cerera ./cmd/cerera

# Перезапуск сервиса
sudo systemctl restart cerera

# Или если используете Docker
docker-compose down
docker-compose build
docker-compose up -d
```

---

## 📝 Полезные команды

### Просмотр работы ноды:

```bash
# Проверка HTTP API
curl http://localhost:8080/status

# Или если настроен JSON-RPC
curl -X POST http://localhost:8080/app \
  -H "Content-Type: application/json" \
  -d '{"method":"getBalance","jsonrpc":"2.0","id":1,"params":["0x..."]}'

# Проверка процесса
ps aux | grep cerera

# Проверка портов
netstat -tlnp | grep -E '8080|31000'
# или
ss -tlnp | grep -E '8080|31000'
```

### Резервное копирование:

```bash
# Создайте скрипт бэкапа
cat > ~/backup-cerera.sh << 'EOF'
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR=~/cerera-backups
mkdir -p $BACKUP_DIR

# Остановка ноды (опционально)
# sudo systemctl stop cerera

# Копирование данных
tar -czf $BACKUP_DIR/cerera-$DATE.tar.gz \
  ~/cerera-data \
  ~/cerera-keys \
  ~/cerera-deploy/cerera/config.json

# Запуск ноды обратно
# sudo systemctl start cerera

echo "Backup completed: $BACKUP_DIR/cerera-$DATE.tar.gz"
EOF

chmod +x ~/backup-cerera.sh

# Добавьте в crontab для автоматического бэкапа
crontab -e
# Добавьте строку: 0 2 * * * /home/YOUR_USERNAME/backup-cerera.sh
```

---

## ⚠️ Возможные проблемы и решения

### Проблема: Нода не запускается

```bash
# Проверьте логи
journalctl -u cerera -n 50
# или
cat ~/cerera-data/cerera-error.log

# Проверьте права доступа
ls -la ~/cerera-deploy/cerera/cerera
chmod +x ~/cerera-deploy/cerera/cerera

# Проверьте порты
sudo lsof -i :8080
sudo lsof -i :31000
```

### Проблема: Порт уже занят

```bash
# Найдите процесс, использующий порт
sudo lsof -i :8080
sudo kill -9 PID

# Или измените порт в конфигурации/параметрах запуска
```

### Проблема: Ошибки с библиотеками RandomX

Если возникают проблемы с компиляцией RandomX:

```bash
# Убедитесь, что установлены необходимые библиотеки
sudo apt install -y gcc g++ make cmake  # Ubuntu/Debian

# Для использования предкомпилированных библиотек
# они находятся в build/linux-x86_64/librandomx.a
```

---

## 📞 Быстрая справка по параметрам запуска

```bash
./cerera [параметры]

Основные параметры:
  -addr string      P2P адрес (default "31000")
  -key string       Путь к PEM ключу
  -mode string      Режим: server, client, p2p (default "server")
  -http int         HTTP порт (default 8080)
  -miner bool       Включить майнинг (default true)
  -mem bool         Хранение в памяти (true) или на диске (false) (default true)

Примеры:
  ./cerera -mode=p2p -addr=31000 -http=8080 -miner=true -mem=false
  ./cerera -mode=p2p -http=1337 -key=./keys/nodekey.pem
```

---

## 🎯 Рекомендуемая структура на сервере

```
~/cerera-deploy/
├── cerera/              # Исходный код
│   ├── cmd/
│   ├── internal/
│   └── cerera          # Исполняемый файл
├── cerera-data/         # Данные блокчейна
│   ├── chain.dat
│   ├── config.json
│   └── cerera.log
└── cerera-keys/         # Ключи ноды
    └── nodekey.pem
```

---

**Готово!** Ваша нода Cerera должна быть запущена и работать на сервере.

Для проверки доступности API откройте в браузере или curl:
```
http://your-server-ip:8080/status
```

