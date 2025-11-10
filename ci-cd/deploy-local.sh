#!/bin/bash

# Скрипт для локального развертывания Cerera на текущем хосте
# Использует значения по умолчанию без флагов командной строки

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Функции для вывода
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Определение директорий
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$HOME/cerera"
DATA_DIR="$HOME/cerera-data"
KEYS_DIR="$HOME/cerera-keys"
BINARY_NAME="cerera"
SERVICE_NAME="cerera"

info "🚀 Развертывание Cerera на локальном хосте"
info "Директория проекта: $PROJECT_DIR"
info "Директория установки: $INSTALL_DIR"
echo ""

# Проверка наличия Go
info "🔍 Проверка установки Go..."
if ! command -v go &> /dev/null; then
    error "Go не установлен!"
    echo "Установите Go 1.23.0 или выше:"
    echo "  wget https://go.dev/dl/go1.23.6.linux-amd64.tar.gz"
    echo "  sudo tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz"
    echo "  export PATH=\$PATH:/usr/local/go/bin"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
info "✓ Go установлен: $GO_VERSION"
echo ""

# Создание директорий
info "📁 Создание директорий..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$KEYS_DIR"
info "✓ Директории созданы"
echo ""

# Копирование файлов проекта
info "📦 Копирование файлов проекта..."
cp -r "$PROJECT_DIR"/* "$INSTALL_DIR/" 2>/dev/null || {
    # Если копирование не удалось, работаем в текущей директории
    warn "Работаем в текущей директории проекта"
    INSTALL_DIR="$PROJECT_DIR"
}
info "✓ Файлы скопированы в $INSTALL_DIR"
echo ""

# Переход в директорию проекта
cd "$INSTALL_DIR"

# Скачивание зависимостей
info "⬇️  Скачивание зависимостей Go..."
go mod download
info "✓ Зависимости установлены"
echo ""

# Сборка проекта
info "🔨 Сборка проекта..."
if go build -o "$INSTALL_DIR/$BINARY_NAME" ./cmd/cerera; then
    info "✓ Сборка успешна: $INSTALL_DIR/$BINARY_NAME"
else
    error "Ошибка при сборке проекта"
    exit 1
fi
echo ""

# Проверка создания бинарника
if [ ! -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    error "Бинарный файл не найден после сборки"
    exit 1
fi

# Установка прав на выполнение
chmod +x "$INSTALL_DIR/$BINARY_NAME"
info "✓ Права на выполнение установлены"
echo ""

# Копирование конфигурации (если существует)
if [ -f "$PROJECT_DIR/config.json" ]; then
    info "⚙️  Копирование конфигурации..."
    cp "$PROJECT_DIR/config.json" "$DATA_DIR/config.json"
    info "✓ Конфигурация скопирована"
    echo ""
fi

# Определение пользователя
CURRENT_USER=$(whoami)
info "Пользователь: $CURRENT_USER"
echo ""

# Создание systemd service файла
info "📝 Создание systemd service..."
SERVICE_FILE="/tmp/$SERVICE_NAME.service"

cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=$CURRENT_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=10
StandardOutput=append:$DATA_DIR/cerera.log
StandardError=append:$DATA_DIR/cerera-error.log

# Ограничения ресурсов (опционально)
# LimitNOFILE=65536
# LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

info "✓ Service файл создан: $SERVICE_FILE"
echo ""

# Установка systemd service
if command -v systemctl &> /dev/null && systemctl --version &> /dev/null; then
    info "🔧 Установка systemd service..."
    
    if [ "$EUID" -eq 0 ]; then
        # Запущено от root
        cp "$SERVICE_FILE" "/etc/systemd/system/$SERVICE_NAME.service"
        systemctl daemon-reload
        systemctl enable "$SERVICE_NAME"
        info "✓ Service установлен и включен"
    else
        # Требуется sudo
        info "Требуются права sudo для установки service..."
        sudo cp "$SERVICE_FILE" "/etc/systemd/system/$SERVICE_NAME.service"
        sudo systemctl daemon-reload
        sudo systemctl enable "$SERVICE_NAME"
        info "✓ Service установлен и включен"
    fi
    echo ""
    
    # Вопрос о запуске сервиса
    read -p "Запустить сервис сейчас? (y/n) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        if [ "$EUID" -eq 0 ]; then
            systemctl start "$SERVICE_NAME"
        else
            sudo systemctl start "$SERVICE_NAME"
        fi
        
        sleep 2
        
        info "📊 Статус сервиса:"
        if [ "$EUID" -eq 0 ]; then
            systemctl status "$SERVICE_NAME" --no-pager -l
        else
            sudo systemctl status "$SERVICE_NAME" --no-pager -l
        fi
    else
        warn "Сервис создан, но не запущен"
        info "Запустите вручную: sudo systemctl start $SERVICE_NAME"
    fi
else
    warn "systemd не найден, создание service пропущено"
    info "Запустите ноду вручную: $INSTALL_DIR/$BINARY_NAME"
fi

echo ""
info "✅ Развертывание завершено!"
echo ""
info "═══════════════════════════════════════"
info "Информация о развертывании:"
info "═══════════════════════════════════════"
echo "  Бинарный файл:    $INSTALL_DIR/$BINARY_NAME"
echo "  Данные:           $DATA_DIR"
echo "  Ключи:            $KEYS_DIR"
echo "  Логи:             $DATA_DIR/cerera.log"
echo "  Ошибки:           $DATA_DIR/cerera-error.log"
echo ""
info "Параметры по умолчанию:"
echo "  Режим:            server"
echo "  P2P адрес:        31000"
echo "  HTTP порт:        8080"
echo "  Майнинг:          включен"
echo "  Хранение:         в памяти"
echo ""
info "Полезные команды:"
echo "  Статус:           sudo systemctl status $SERVICE_NAME"
echo "  Логи:             sudo journalctl -u $SERVICE_NAME -f"
echo "  Логи (файл):      tail -f $DATA_DIR/cerera.log"
echo "  Перезапуск:       sudo systemctl restart $SERVICE_NAME"
echo "  Остановка:        sudo systemctl stop $SERVICE_NAME"
echo "  Проверка API:     curl http://localhost:8080/status"
echo ""
info "Проверка работы:"
echo "  curl http://localhost:8080/status"
echo "  netstat -tlnp | grep -E '8080|31000'"
echo ""

