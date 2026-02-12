#!/bin/bash
# Скрипт очистки Docker на Ubuntu
# Удаляет остановленные контейнеры, неиспользуемые образы, тома и кэш сборки

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Режимы: light (без образов и томов), full (всё), volumes (с томами)
MODE="${1:-light}"
FORCE="${2:-}"

usage() {
    echo "Использование: $0 [light|full|volumes] [-y]"
    echo ""
    echo "  light   - остановленные контейнеры, висячие образы, неиспользуемые сети (по умолчанию)"
    echo "  full    - light + все неиспользуемые образы + тома + кэш сборки"
    echo "  volumes - light + неиспользуемые тома (образы не трогаем)"
    echo ""
    echo "  -y      - без подтверждения"
    exit 1
}

case "$MODE" in
    light|full|volumes) ;;
    -y) FORCE="-y"; MODE="light" ;;
    *) usage ;;
esac

[[ "$2" == "-y" ]] && FORCE="-y"

# Проверка наличия Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker не найден. Установите Docker.${NC}"
    exit 1
fi

# Проверка прав
if ! docker info &> /dev/null; then
    echo -e "${RED}Нет доступа к Docker. Запустите скрипт с sudo или добавьте пользователя в группу docker.${NC}"
    exit 1
fi

echo -e "${CYAN}=== Очистка Docker (режим: $MODE) ===${NC}"

# Показать текущее использование места
echo -e "\n${YELLOW}До очистки:${NC}"
docker system df

confirm() {
    if [[ "$FORCE" != "-y" ]]; then
        echo -e "\n${YELLOW}Продолжить? [y/N]${NC}"
        read -r ans
        [[ "$ans" != "y" && "$ans" != "Y" ]] && echo "Отменено." && exit 0
    fi
}

case "$MODE" in
    light)
        confirm
        echo -e "\n${GREEN}Удаление остановленных контейнеров...${NC}"
        docker container prune -f
        echo -e "${GREEN}Удаление висячих образов (<none>)...${NC}"
        docker image prune -f
        echo -e "${GREEN}Удаление неиспользуемых сетей...${NC}"
        docker network prune -f
        ;;
    volumes)
        confirm
        echo -e "\n${GREEN}Удаление остановленных контейнеров...${NC}"
        docker container prune -f
        echo -e "${GREEN}Удаление висячих образов...${NC}"
        docker image prune -f
        echo -e "${GREEN}Удаление неиспользуемых сетей...${NC}"
        docker network prune -f
        echo -e "${GREEN}Удаление неиспользуемых томов...${NC}"
        docker volume prune -f
        ;;
    full)
        confirm
        echo -e "\n${GREEN}Полная очистка (контейнеры, образы, тома, сети, кэш сборки)...${NC}"
        docker system prune -a -f --volumes
        ;;
esac

echo -e "\n${YELLOW}После очистки:${NC}"
docker system df
echo -e "\n${GREEN}Готово.${NC}"
