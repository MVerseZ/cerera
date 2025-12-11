# Скрипт для удаления Docker образов с именем <none> (dangling images)

Write-Host "Поиск Docker образов с именем <none>..." -ForegroundColor Yellow

# Получаем все образы с именем <none>
$danglingImages = docker images --filter "dangling=true" -q

if ($null -eq $danglingImages -or $danglingImages.Count -eq 0) {
    Write-Host "Не найдено образов с именем <none>" -ForegroundColor Green
    exit 0
}

Write-Host "Найдено образов для удаления: $($danglingImages.Count)" -ForegroundColor Cyan

# Показываем список образов перед удалением
Write-Host "`nСписок образов для удаления:" -ForegroundColor Yellow
docker images --filter "dangling=true"

# Подтверждение удаления
$confirm = Read-Host "`nУдалить эти образы? (y/n)"
if ($confirm -ne "y" -and $confirm -ne "Y") {
    Write-Host "Удаление отменено" -ForegroundColor Yellow
    exit 0
}

# Удаляем образы
Write-Host "`nУдаление образов..." -ForegroundColor Yellow
docker rmi $danglingImages --force

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nОбразы успешно удалены!" -ForegroundColor Green
} else {
    Write-Host "`nПроизошла ошибка при удалении образов" -ForegroundColor Red
    exit 1
}

# Альтернативный вариант: удаление всех образов с <none> в имени (более агрессивный)
# Раскомментируйте, если нужно удалить все образы, где REPOSITORY или TAG = <none>
# Write-Host "`nПоиск всех образов с <none> в имени..." -ForegroundColor Yellow
# $allNoneImages = docker images | Where-Object { $_ -match '<none>' } | ForEach-Object { ($_ -split '\s+')[2] } | Select-Object -Unique
# if ($allNoneImages) {
#     docker rmi $allNoneImages --force
# }

