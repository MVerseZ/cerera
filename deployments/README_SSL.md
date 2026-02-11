# Настройка SSL/TLS сертификатов

Этот проект поддерживает HTTPS через SSL/TLS сертификаты. Есть два способа получения сертификатов:

## 1. Самоподписанный сертификат (для разработки)

### Linux/Mac:
```bash
cd ci-cd
chmod +x generate_cert.sh
./generate_cert.sh
```

### Windows:
```cmd
cd ci-cd
generate_cert.bat
```

**Примечание:** Самоподписанные сертификаты вызывают предупреждения в браузерах и подходят только для разработки.

## 2. Сертификат от Let's Encrypt (для продакшена)

### Требования:
- Домен, указывающий на ваш сервер
- Открытый порт 80 (для HTTP-01 challenge)
- Права администратора
- Установленный certbot

### Установка certbot:

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install certbot
```

**CentOS/RHEL:**
```bash
sudo yum install certbot
```

**macOS:**
```bash
brew install certbot
```

### Получение сертификата:

**Linux/Mac:**
```bash
cd ci-cd
chmod +x get_letsencrypt_cert.sh
sudo ./get_letsencrypt_cert.sh yourdomain.com your@email.com
```

**Важно:** Перед запуском убедитесь, что:
- Порт 80 свободен (остановите веб-сервер)
- Домен указывает на IP-адрес вашего сервера
- Firewall разрешает входящие соединения на порт 80

### Создание симлинков на сертификаты:

После получения сертификата создайте симлинки в корне проекта:

```bash
cd ci-cd
chmod +x link_letsencrypt_cert.sh
./link_letsencrypt_cert.sh yourdomain.com
```

Или вручную:
```bash
cd /path/to/project
sudo ln -s /etc/letsencrypt/live/yourdomain.com/fullchain.pem ./server.crt
sudo ln -s /etc/letsencrypt/live/yourdomain.com/privkey.pem ./server.key
```

### Права доступа:

Если приложение не может прочитать сертификаты, настройте права:

```bash
# Вариант 1: Добавить пользователя в группу ssl-cert
sudo usermod -a -G ssl-cert $USER

# Вариант 2: Изменить права на директории
sudo chmod 755 /etc/letsencrypt/live
sudo chmod 755 /etc/letsencrypt/archive
```

### Автоматическое обновление сертификатов:

Let's Encrypt сертификаты действительны 90 дней. Настройте автоматическое обновление:

**Через systemd timer (рекомендуется):**
```bash
sudo systemctl enable certbot.timer
sudo systemctl start certbot.timer
```

**Через crontab:**
```bash
# Добавьте в crontab (crontab -e):
0 0 * * * certbot renew --quiet && systemctl reload your-service
```

### Альтернативные методы получения сертификата:

#### 1. DNS-01 challenge (без остановки сервера):
```bash
certbot certonly --manual --preferred-challenges dns -d yourdomain.com
```

#### 2. Webroot (если сервер уже работает):
```bash
certbot certonly --webroot -w /var/www/html -d yourdomain.com
```

#### 3. Через nginx/apache плагин:
```bash
# Nginx
sudo certbot --nginx -d yourdomain.com

# Apache
sudo certbot --apache -d yourdomain.com
```

## Настройка в приложении

После получения сертификатов включите TLS в конфигурации:

1. Откройте `config.json`
2. Установите `SEC.HTTP.TLS` в `true`
3. Убедитесь, что файлы `server.crt` и `server.key` находятся в корне проекта

## Проверка сертификата

После запуска сервера проверьте сертификат:

```bash
openssl s_client -connect localhost:8080 -servername yourdomain.com
```

Или откройте в браузере: `https://yourdomain.com:8080`

## Troubleshooting

### Ошибка "permission denied" при чтении сертификатов:
- Проверьте права доступа к файлам сертификатов
- Убедитесь, что пользователь, под которым запущено приложение, имеет доступ

### Ошибка "certificate verify failed":
- Проверьте, что сертификат не истек
- Убедитесь, что домен в сертификате совпадает с доменом запроса

### Certbot не может пройти валидацию:
- Убедитесь, что порт 80 открыт и доступен из интернета
- Проверьте, что домен указывает на правильный IP-адрес
- Используйте DNS-01 challenge, если порт 80 недоступен

