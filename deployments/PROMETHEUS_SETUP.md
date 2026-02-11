# Запуск Cerera с Prometheus через Docker Compose

Это руководство описывает, как запустить кластер Cerera с мониторингом через Prometheus и Grafana.

## 🚀 Быстрый старт

### Вариант 1: Одна нода + Prometheus + Grafana (для тестирования)

```bash
cd ci-cd
docker-compose -f docker-compose-single.yml up -d
```

Этот вариант запускает:
- 1 нода Cerera (node1)
- Prometheus для сбора метрик
- Grafana для визуализации

**Идеально для:**
- Локальной разработки
- Тестирования
- Минимальных ресурсов

### Вариант 2: Полный стек (5 нод + Prometheus + Grafana)

```bash
cd ci-cd
docker-compose -f docker-compose-full.yml up -d
```

Этот вариант запускает:
- 5 нод Cerera (node1-node5)
- Prometheus для сбора метрик
- Grafana для визуализации

### Вариант 3: Только мониторинг (Prometheus + Grafana)

Если ноды уже запущены отдельно:

```bash
cd ci-cd
docker-compose up -d
```

Этот вариант запускает только Prometheus и Grafana, которые будут собирать метрики с нод, запущенных на хосте.

## 📊 Доступ к сервисам

После запуска доступны следующие сервисы:

- **Prometheus UI**: http://localhost:9090
- **Grafana**: http://localhost:3100
  - Логин: `admin`
  - Пароль: `admin`
- **Cerera Node 1**: http://localhost:1337
  - Метрики: http://localhost:1337/metrics

**Для варианта с 5 нодами также доступны:**
- **Cerera Node 2**: http://localhost:1338
- **Cerera Node 3**: http://localhost:1339
- **Cerera Node 4**: http://localhost:1340
- **Cerera Node 5**: http://localhost:1341

## 🔍 Проверка метрик

### Проверка метрик ноды напрямую

```bash
# Метрики node1
curl http://localhost:1337/metrics

# Метрики node2
curl http://localhost:1338/metrics
```

### Проверка в Prometheus

1. Откройте http://localhost:9090
2. Перейдите в Status → Targets
3. Убедитесь, что все ноды в состоянии "UP"

### Проверка в Grafana

1. Откройте http://localhost:3100
2. Войдите с учетными данными admin/admin
3. Перейдите в Dashboards → Browse
4. Выберите дашборд Cerera (если он настроен)

## 🛠️ Настройка

### Изменение количества нод

Если нужно изменить количество нод:

1. Отредактируйте `docker-compose-full.yml` - добавьте/удалите сервисы nodeN
2. Обновите `prometheus.yml` - добавьте/удалите targets для новых нод
3. Перезапустите:

```bash
docker-compose -f docker-compose-full.yml down
docker-compose -f docker-compose-full.yml up -d
```

### Настройка интервала сбора метрик

**Для одной ноды** - отредактируйте `prometheus-single.yml`:
```yaml
global:
  scrape_interval: 5s  # Интервал сбора метрик
  evaluation_interval: 5s  # Интервал оценки правил
```

**Для 5 нод** - отредактируйте `prometheus.yml`:
```yaml
global:
  scrape_interval: 5s  # Интервал сбора метрик
  evaluation_interval: 5s  # Интервал оценки правил
```

### Добавление дополнительных нод

Для добавления большего количества нод используйте существующие файлы:
- `docker-compose-9nodes.yml` - для 9 нод
- `docker-compose-15nodes.yml` - для 15 нод

И обновите `prometheus.yml` соответственно.

## 🧹 Остановка и очистка

### Остановка сервисов

**Для одной ноды:**
```bash
docker-compose -f docker-compose-single.yml down
```

**Для 5 нод:**
```bash
docker-compose -f docker-compose-full.yml down
```

### Остановка с удалением данных

**Для одной ноды:**
```bash
docker-compose -f docker-compose-single.yml down -v
```

**Для 5 нод:**
```bash
docker-compose -f docker-compose-full.yml down -v
```

⚠️ **Внимание**: Это удалит все данные, включая историю метрик в Prometheus и дашборды Grafana.

## 📝 Логи

### Просмотр логов всех сервисов

**Для одной ноды:**
```bash
docker-compose -f docker-compose-single.yml logs -f
```

**Для 5 нод:**
```bash
docker-compose -f docker-compose-full.yml logs -f
```

### Просмотр логов конкретного сервиса

**Для одной ноды:**
```bash
# Логи Prometheus
docker-compose -f docker-compose-single.yml logs -f prometheus

# Логи ноды
docker-compose -f docker-compose-single.yml logs -f node1

# Логи Grafana
docker-compose -f docker-compose-single.yml logs -f grafana
```

**Для 5 нод:**
```bash
# Логи Prometheus
docker-compose -f docker-compose-full.yml logs -f prometheus

# Логи конкретной ноды
docker-compose -f docker-compose-full.yml logs -f node1

# Логи Grafana
docker-compose -f docker-compose-full.yml logs -f grafana
```

## 🔧 Устранение проблем

### Prometheus не видит ноды

1. Проверьте, что все ноды запущены:
   ```bash
   # Для одной ноды
   docker-compose -f docker-compose-single.yml ps
   
   # Для 5 нод
   docker-compose -f docker-compose-full.yml ps
   ```

2. Проверьте, что ноды находятся в той же сети:
   ```bash
   docker network inspect ci-cd_cerera-network
   ```

3. Проверьте доступность метрик напрямую:
   ```bash
   docker exec cerera-node1 curl http://localhost:1337/metrics
   ```

### Метрики не отображаются в Grafana

1. Проверьте, что Prometheus работает: http://localhost:9090
2. Проверьте настройки datasource в Grafana
3. Убедитесь, что дашборды правильно настроены

### Проблема с входом в Grafana (Invalid username or password)

Если вы получаете ошибку "Login failed - Invalid username or password", это означает, что Grafana уже была запущена ранее с другими учетными данными. Переменные окружения `GF_SECURITY_ADMIN_USER` и `GF_SECURITY_ADMIN_PASSWORD` работают только при первом запуске.

**Решение 1: Использование скрипта для сброса пароля (самый простой способ)**

**Windows (PowerShell):**
```powershell
cd ci-cd
.\reset-grafana-password.ps1
# Или с указанием нового пароля:
.\reset-grafana-password.ps1 "мой_новый_пароль"
```

**Linux/Mac:**
```bash
cd ci-cd
chmod +x reset-grafana-password.sh
./reset-grafana-password.sh
# Или с указанием нового пароля:
./reset-grafana-password.sh "мой_новый_пароль"
```

**Решение 2: Сброс пароля через команду вручную**

```bash
# Остановите Grafana
docker-compose -f docker-compose-full.yml stop grafana

# Сбросьте пароль администратора
docker exec -it cerera-grafana grafana-cli admin reset-admin-password admin

# Или если контейнер остановлен, запустите временный контейнер
docker run --rm -v ci-cd_grafana_data:/var/lib/grafana grafana/grafana:latest grafana-cli admin reset-admin-password admin

# Запустите Grafana снова
docker-compose -f docker-compose-full.yml start grafana
```

**Решение 3: Удаление данных Grafana (удалит все настройки и дашборды)**

```bash
# Остановите все сервисы
docker-compose -f docker-compose-full.yml down

# Удалите только volume Grafana
docker volume rm ci-cd_grafana_data

# Запустите снова
docker-compose -f docker-compose-full.yml up -d
```

**Решение 4: Полная очистка (удалит все данные, включая Prometheus)**

```bash
# Остановите все сервисы и удалите все volumes
docker-compose -f docker-compose-full.yml down -v

# Запустите снова
docker-compose -f docker-compose-full.yml up -d
```

После любого из решений используйте:
- **Логин**: `admin`
- **Пароль**: `admin`

## 📚 Дополнительная информация

- [Официальная документация Prometheus](https://prometheus.io/docs/)
- [Официальная документация Grafana](https://grafana.com/docs/)
- [README.md](./README.md) - общая информация о деплое
