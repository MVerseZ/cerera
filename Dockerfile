# Этап сборки
FROM golang:1.25-rc-alpine AS builder

# Устанавливаем зависимости
RUN apk add --no-cache git make gcc musl-dev

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

# Копируем файл ключа (если он должен быть в контейнере)
# Предполагаем, что nodekey.pem находится в текущей директории при сборке
COPY ddddd.nodekey.pem /etc/cerera/ddddd.nodekey.pem

# Рабочая директория
WORKDIR /app

# Открываем порты (если нужно)
EXPOSE 1337 31000

# Команда запуска
CMD ["cerera", "--key=/etc/cerera/${nodekey}", "--mode=p2p", "--http=1337", "--mem=true", "--miner"]