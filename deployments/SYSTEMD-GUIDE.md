# Руководство по созданию systemd service через терминал

## 📋 Основные шаги

### 1. Создание unit файла

Создайте файл `.service` в директории `/etc/systemd/system/`:

```bash
sudo nano /etc/systemd/system/cerera.service
```

Или используйте `tee` для создания через терминал:

```bash
sudo tee /etc/systemd/system/cerera.service > /dev/null << 'EOF'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=your_username
WorkingDirectory=/home/your_username/cerera
ExecStart=/home/your_username/cerera/cerera
Restart=always
RestartSec=10
StandardOutput=append:/home/your_username/cerera-data/cerera.log
StandardError=append:/home/your_username/cerera-data/cerera-error.log

[Install]
WantedBy=multi-user.target
EOF
```

**Замена:** `your_username` на ваше имя пользователя.

---

## 🔧 Быстрый способ через одну команду

### Полная команда для Cerera:

```bash
sudo tee /etc/systemd/system/cerera.service > /dev/null << 'EOF'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=$(whoami)
WorkingDirectory=$HOME/cerera
ExecStart=$HOME/cerera/cerera
Restart=always
RestartSec=10
StandardOutput=append=$HOME/cerera-data/cerera.log
StandardError=append=$HOME/cerera-data/cerera-error.log

[Install]
WantedBy=multi-user.target
EOF
```

**Примечание:** Переменные `$(whoami)` и `$HOME` нужно заменить реальными значениями или использовать скрипт.

---

## 📝 Пошаговая инструкция

### Шаг 1: Создание файла service

**Вариант A: Через nano/vim**
```bash
sudo nano /etc/systemd/system/cerera.service
```
Вставьте содержимое service файла, сохраните (Ctrl+O, Enter, Ctrl+X).

**Вариант B: Через cat с heredoc**
```bash
sudo bash -c 'cat > /etc/systemd/system/cerera.service' << 'EOF'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=username
WorkingDirectory=/home/username/cerera
ExecStart=/home/username/cerera/cerera
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
```

**Вариант C: Через echo (для простых случаев)**
```bash
echo '[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
ExecStart=/home/username/cerera/cerera
Restart=always

[Install]
WantedBy=multi-user.target' | sudo tee /etc/systemd/system/cerera.service
```

---

### Шаг 2: Перезагрузка systemd

После создания файла нужно перезагрузить systemd, чтобы он узнал о новом сервисе:

```bash
sudo systemctl daemon-reload
```

---

### Шаг 3: Включение автозапуска

Включите автозапуск сервиса при загрузке системы:

```bash
sudo systemctl enable cerera.service
```

Или просто:
```bash
sudo systemctl enable cerera
```

---

### Шаг 4: Запуск сервиса

Запустите сервис:

```bash
sudo systemctl start cerera
```

---

### Шаг 5: Проверка статуса

Проверьте, что сервис работает:

```bash
sudo systemctl status cerera
```

---

## 🎯 Полный пример создания для Cerera

Скопируйте и выполните все команды подряд (замените `username` на ваше имя пользователя):

```bash
# 1. Создание service файла
sudo tee /etc/systemd/system/cerera.service > /dev/null << 'EOFSERVICE'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=username
WorkingDirectory=/home/username/cerera
ExecStart=/home/username/cerera/cerera
Restart=always
RestartSec=10
StandardOutput=append=/home/username/cerera-data/cerera.log
StandardError=append=/home/username/cerera-data/cerera-error.log

[Install]
WantedBy=multi-user.target
EOFSERVICE

# 2. Перезагрузка systemd
sudo systemctl daemon-reload

# 3. Включение автозапуска
sudo systemctl enable cerera

# 4. Запуск сервиса
sudo systemctl start cerera

# 5. Проверка статуса
sudo systemctl status cerera
```

**С автоматической подстановкой имени пользователя:**

```bash
USERNAME=$(whoami)
HOME_DIR=$(eval echo ~$USERNAME)

sudo tee /etc/systemd/system/cerera.service > /dev/null << EOFSERVICE
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=$USERNAME
WorkingDirectory=$HOME_DIR/cerera
ExecStart=$HOME_DIR/cerera/cerera
Restart=always
RestartSec=10
StandardOutput=append=$HOME_DIR/cerera-data/cerera.log
StandardError=append=$HOME_DIR/cerera-data/cerera-error.log

[Install]
WantedBy=multi-user.target
EOFSERVICE

sudo systemctl daemon-reload
sudo systemctl enable cerera
sudo systemctl start cerera
sudo systemctl status cerera
```

---

## 📖 Структура service файла

### Основные секции:

#### [Unit]
- `Description` - описание сервиса
- `After` - запускать после указанных сервисов (например, `network.target`)

#### [Service]
- `Type` - тип сервиса:
  - `simple` - основная команда запускается как главный процесс
  - `forking` - процесс форкается, родитель завершается
  - `oneshot` - выполняется один раз и завершается
  
- `User` - пользователь, от имени которого запускается (обязательно для безопасности)
- `WorkingDirectory` - рабочая директория
- `ExecStart` - команда запуска (полный путь к бинарнику)
- `ExecStop` - команда остановки (опционально)
- `Restart` - политика перезапуска:
  - `always` - всегда перезапускать
  - `on-failure` - перезапускать при ошибке
  - `no` - не перезапускать
  
- `RestartSec` - задержка перед перезапуском (в секундах)
- `StandardOutput` - куда перенаправлять stdout
- `StandardError` - куда перенаправлять stderr

#### [Install]
- `WantedBy` - в какой target включать (обычно `multi-user.target`)

---

## 🔍 Полезные команды управления

### Просмотр логов:
```bash
# Логи через journalctl
sudo journalctl -u cerera -f          # Следить за логами в реальном времени
sudo journalctl -u cerera -n 50       # Последние 50 строк
sudo journalctl -u cerera --since today

# Логи из файла (если настроен StandardOutput)
tail -f ~/cerera-data/cerera.log
```

### Управление сервисом:
```bash
sudo systemctl start cerera           # Запустить
sudo systemctl stop cerera            # Остановить
sudo systemctl restart cerera         # Перезапустить
sudo systemctl reload cerera          # Перезагрузить конфигурацию (если поддерживается)
sudo systemctl status cerera          # Статус
```

### Информация:
```bash
sudo systemctl is-enabled cerera      # Проверить автозапуск
sudo systemctl is-active cerera       # Проверить активность
sudo systemctl list-units --type=service | grep cerera
```

### Отключение и удаление:
```bash
sudo systemctl stop cerera            # Остановить
sudo systemctl disable cerera        # Отключить автозапуск
sudo rm /etc/systemd/system/cerera.service  # Удалить файл
sudo systemctl daemon-reload          # Перезагрузить systemd
```

---

## 🛠️ Примеры для разных случаев

### С параметрами командной строки:

```bash
sudo tee /etc/systemd/system/cerera.service > /dev/null << 'EOF'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/CERERA_CHAIN/cerera
ExecStart=/root/CERERA_CHAIN/cerera/cerera -mode=p2p -addr=31000 -http=1337 -mem -mine
Restart=always
RestartSec=10
LimitNOFILE=65536
LimitNPROC=4096
MemoryMax=2G
CPUQuota=50%

[Install]
WantedBy=multi-user.target
EOF
```

### С переменными окружения:

```bash
sudo tee /etc/systemd/system/cerera.service > /dev/null << 'EOF'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=username
Environment="CERERA_MODE=p2p"
Environment="CERERA_HTTP_PORT=8080"
WorkingDirectory=/home/username/cerera
ExecStart=/home/username/cerera/cerera
Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

### С ограничениями ресурсов:

```bash
sudo tee /etc/systemd/system/cerera.service > /dev/null << 'EOF'
[Unit]
Description=Cerera Blockchain Node
After=network.target

[Service]
Type=simple
User=username
WorkingDirectory=/home/username/cerera
ExecStart=/home/username/cerera/cerera
Restart=always
LimitNOFILE=65536
LimitNPROC=4096
MemoryMax=2G
CPUQuota=50%

[Install]
WantedBy=multi-user.target
EOF
```

### Запуск после другого сервиса:

```bash
sudo tee /etc/systemd/system/cerera.service > /dev/null << 'EOF'
[Unit]
Description=Cerera Blockchain Node
After=network.target postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=username
ExecStart=/home/username/cerera/cerera
Restart=always

[Install]
WantedBy=multi-user.target
EOF
```

---

## ⚠️ Частые ошибки и решения

### Ошибка: "Failed to start: Unit cerera.service not found"
**Решение:** Выполните `sudo systemctl daemon-reload` после создания файла

### Ошибка: "Permission denied"
**Решение:** Убедитесь, что:
- Используете `sudo` для создания файла
- Указали правильного пользователя в `User=`
- Бинарный файл имеет права на выполнение: `chmod +x /path/to/cerera`

### Ошибка: "WorkingDirectory is not a directory"
**Решение:** Проверьте, что директория существует:
```bash
mkdir -p /home/username/cerera
```

### Сервис запускается, но сразу останавливается
**Решение:** Проверьте логи:
```bash
sudo journalctl -u cerera -n 50
# или
tail -f ~/cerera-data/cerera-error.log
```

---

## ✅ Проверочный чеклист

После создания service файла:

- [ ] Файл создан: `ls -l /etc/systemd/system/cerera.service`
- [ ] Выполнен `sudo systemctl daemon-reload`
- [ ] Выполнен `sudo systemctl enable cerera`
- [ ] Сервис запущен: `sudo systemctl start cerera`
- [ ] Статус активен: `sudo systemctl status cerera` показывает "active (running)"
- [ ] Логи работают: `sudo journalctl -u cerera -f` показывает вывод
- [ ] Автозапуск включен: `sudo systemctl is-enabled cerera` показывает "enabled"

---

## 🎓 Дополнительная информация

- Документация systemd: `man systemd.service`
- Документация systemctl: `man systemctl`
- Все unit файлы: `/etc/systemd/system/`
- Пользовательские unit файлы: `~/.config/systemd/user/`
- Проверка синтаксиса файла: `systemd-analyze verify /etc/systemd/system/cerera.service`

