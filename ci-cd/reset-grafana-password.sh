#!/bin/bash
# Скрипт для сброса пароля администратора Grafana
# Использование: ./reset-grafana-password.sh [новый_пароль]

NEW_PASSWORD=${1:-admin}

echo "Сброс пароля администратора Grafana..."

# Проверяем, запущен ли контейнер Grafana
if docker ps --filter "name=cerera-grafana" --format "{{.Names}}" | grep -q cerera-grafana; then
    echo "Контейнер Grafana запущен. Останавливаем..."
    docker stop cerera-grafana
    
    echo "Сброс пароля..."
    docker run --rm -v ci-cd_grafana_data:/var/lib/grafana grafana/grafana:latest grafana-cli admin reset-admin-password "$NEW_PASSWORD"
    
    echo "Запуск Grafana..."
    docker start cerera-grafana
    
    echo "Пароль успешно сброшен!"
    echo "Логин: admin"
    echo "Пароль: $NEW_PASSWORD"
else
    echo "Контейнер Grafana не запущен. Сброс пароля..."
    
    # Проверяем, существует ли volume
    if docker volume ls --filter "name=grafana_data" --format "{{.Name}}" | grep -q grafana_data; then
        docker run --rm -v ci-cd_grafana_data:/var/lib/grafana grafana/grafana:latest grafana-cli admin reset-admin-password "$NEW_PASSWORD"
        echo "Пароль успешно сброшен!"
        echo "Логин: admin"
        echo "Пароль: $NEW_PASSWORD"
        echo "Теперь запустите Grafana: docker-compose up -d grafana"
    else
        echo "Volume grafana_data не найден. При первом запуске пароль будет установлен автоматически."
    fi
fi
