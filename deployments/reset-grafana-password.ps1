# Скрипт для сброса пароля администратора Grafana
# Использование: .\reset-grafana-password.ps1 [новый_пароль]

param(
    [string]$NewPassword = "admin"
)

Write-Host "Сброс пароля администратора Grafana..." -ForegroundColor Yellow

# Проверяем, запущен ли контейнер Grafana
$containerRunning = docker ps --filter "name=cerera-grafana" --format "{{.Names}}"

if ($containerRunning) {
    Write-Host "Контейнер Grafana запущен. Останавливаем..." -ForegroundColor Yellow
    docker stop cerera-grafana
    
    Write-Host "Сброс пароля..." -ForegroundColor Yellow
    docker run --rm -v ci-cd_grafana_data:/var/lib/grafana grafana/grafana:latest grafana-cli admin reset-admin-password $NewPassword
    
    Write-Host "Запуск Grafana..." -ForegroundColor Yellow
    docker start cerera-grafana
    
    Write-Host "Пароль успешно сброшен!" -ForegroundColor Green
    Write-Host "Логин: admin" -ForegroundColor Cyan
    Write-Host "Пароль: $NewPassword" -ForegroundColor Cyan
} else {
    Write-Host "Контейнер Grafana не запущен. Сброс пароля..." -ForegroundColor Yellow
    
    # Проверяем, существует ли volume
    $volumeExists = docker volume ls --filter "name=grafana_data" --format "{{.Name}}"
    
    if ($volumeExists) {
        docker run --rm -v ci-cd_grafana_data:/var/lib/grafana grafana/grafana:latest grafana-cli admin reset-admin-password $NewPassword
        Write-Host "Пароль успешно сброшен!" -ForegroundColor Green
        Write-Host "Логин: admin" -ForegroundColor Cyan
        Write-Host "Пароль: $NewPassword" -ForegroundColor Cyan
        Write-Host "Теперь запустите Grafana: docker-compose up -d grafana" -ForegroundColor Yellow
    } else {
        Write-Host "Volume grafana_data не найден. При первом запуске пароль будет установлен автоматически." -ForegroundColor Yellow
    }
}
