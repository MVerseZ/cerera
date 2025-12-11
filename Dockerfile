# Этап сборки
FROM golang:alpine AS builder

# Устанавливаем зависимости
RUN apk add --no-cache git make gcc musl-dev

# Устанавливаем GOTOOLCHAIN=auto для автоматического выбора версии инструментов
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Копируем файлы go.mod и go.sum (если есть)
COPY go.mod go.sum ./

# Копируем исходный код
COPY . .

# Собираем приложение
RUN go build -v ./...
RUN go build -v -o /app/cerera ./cmd/cerera

# Этап запуска
FROM alpine:latest

ENV nodekey="/etc/cerera/ddddd.nodekey.pem"

# Устанавливаем зависимости для runtime (если нужны)
RUN apk add --no-cache ca-certificates

# Копируем собранный бинарник из этапа сборки
COPY --from=builder /app/cerera /usr/local/bin/cerera

# Создаем директорию для ключа и копируем файл из builder stage
RUN mkdir -p /etc/cerera
# COPY --from=builder /app/ddddd.nodekey.pem /etc/cerera/ddddd.nodekey.pem

# Рабочая директория
WORKDIR /app

# Открываем порты (если нужно)
EXPOSE 1337 31000

# Команда запуска
# Используем shell форму для правильной подстановки переменной окружения
CMD ["sh", "-c", "cerera --key=/etc/cerera/ddddd.nodekey.pem --mode=p2p --http=1337 --mem=true --miner"]