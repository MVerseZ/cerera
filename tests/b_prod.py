#!/usr/bin/env python3
"""
Скрипт для проверки целостности блокчейна:
- Получает высоту блокчейна
- Перебирает все блоки
- Проверяет, что хэши цепочки совпадают (prevHash == hash предыдущего блока)
- Проверяет, что хэши блоков не дублируются
"""

import requests
import json
import sys
import time
from requests.exceptions import ConnectionError, Timeout

def get_chain_height(api_url: str, max_retries: int = 5) -> int:
    """Получает высоту блокчейна с повторными попытками"""
    data_req = {
        "method": "cerera.chain.getInfo",
        "jsonrpc": "2.0",
        "id": 1,
        "params": []
    }
    
    for attempt in range(max_retries):
        try:
            response = requests.post(api_url, json=data_req, timeout=30)
            if response.status_code == 200:
                result = response.json()
                chain_info = result.get('result', {})
                if isinstance(chain_info, dict):
                    # getInfo возвращает объект с полем total
                    height = chain_info.get('total', 0)
                    return int(height)
                else:
                    # Если result - это число напрямую
                    return int(chain_info)
            else:
                if attempt < max_retries - 1:
                    wait_time = 2 ** attempt
                    print(f"⚠️  HTTP {response.status_code}, повтор через {wait_time} сек...")
                    time.sleep(wait_time)
                    continue
                print(f"❌ Ошибка получения высоты: {response.text}")
                return -1
        except (ConnectionError, Timeout) as e:
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt  # Экспоненциальная задержка: 1, 2, 4, 8, 16 сек
                print(f"⚠️  Ошибка подключения (попытка {attempt + 1}/{max_retries}), повтор через {wait_time} сек...")
                time.sleep(wait_time)
                continue
            else:
                print(f"❌ Исключение при получении высоты после {max_retries} попыток: {e}")
                return -1
        except Exception as e:
            print(f"❌ Исключение при получении высоты: {e}")
            return -1
    
    return -1

def get_block_by_index(api_url: str, index: int, max_retries: int = 5) -> dict:
    """Получает блок по индексу с повторными попытками"""
    data_req = {
        "method": "cerera.chain.getBlockByIndex",
        "jsonrpc": "2.0",
        "id": 2,
        "params": [index]
    }
    
    for attempt in range(max_retries):
        try:
            response = requests.post(api_url, json=data_req, timeout=30)
            if response.status_code == 200:
                result = response.json()
                block = result.get('result')
                return block if block else {}
            else:
                if attempt < max_retries - 1:
                    wait_time = 2 ** attempt
                    print(f"\n⚠️  HTTP {response.status_code} для блока {index}, повтор через {wait_time} сек...")
                    time.sleep(wait_time)
                    continue
                print(f"❌ Ошибка получения блока {index}: {response.text}")
                return {}
        except (ConnectionError, Timeout) as e:
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt  # Экспоненциальная задержка: 1, 2, 4, 8, 16 сек
                print(f"\n⚠️  Ошибка подключения для блока {index} (попытка {attempt + 1}/{max_retries}), повтор через {wait_time} сек...")
                time.sleep(wait_time)
                continue
            else:
                print(f"\n❌ Исключение при получении блока {index} после {max_retries} попыток: {e}")
                return {}
        except Exception as e:
            if attempt < max_retries - 1:
                wait_time = 2 ** attempt
                print(f"\n⚠️  Ошибка для блока {index} (попытка {attempt + 1}/{max_retries}), повтор через {wait_time} сек...")
                time.sleep(wait_time)
                continue
            print(f"\n❌ Исключение при получении блока {index}: {e}")
            return {}
    
    return {}

def normalize_hash(hash_value) -> str:
    """Нормализует хэш к строке (обрабатывает разные форматы)"""
    if hash_value is None:
        return ""
    if isinstance(hash_value, str):
        # Убираем префикс 0x если есть
        return hash_value.replace("0x", "").lower()
    if isinstance(hash_value, dict):
        # Если хэш представлен как объект с полями
        if "hex" in hash_value:
            return normalize_hash(hash_value["hex"])
        if "hash" in hash_value:
            return normalize_hash(hash_value["hash"])
    return str(hash_value).lower()

def check_blockchain_integrity(api_url: str = "http://91.199.32.125:1337/app") -> bool:
    """Проверяет целостность блокчейна"""
    print("🔍 Проверка целостности блокчейна")
    print(f"API URL: {api_url}")
    print("=" * 60)
    
    # Получаем высоту
    height = get_chain_height(api_url)
    if height < 0:
        print("❌ Не удалось получить высоту блокчейна")
        return False
    
    if height == 0:
        print("⚠️  Блокчейн пуст (высота = 0)")
        return True
    
    print(f"📊 Высота блокчейна: {height}")
    print(f"📦 Проверка {height} блоков...")
    print("=" * 60)
    
    previous_hash = None
    errors = 0
    seen_hashes = set()  # Множество для отслеживания уже встреченных хэшей
    
    for i in range(height):
        print(f"Проверка блока {i}/{height-1}...", end=" ")
        
        block = get_block_by_index(api_url, i)
        if not block:
            print(f"❌ Блок {i} не найден")
            errors += 1
            continue
        
        # Получаем хэш текущего блока
        current_hash = normalize_hash(block.get("hash"))
        
        # Проверка на дублирование хэшей
        if current_hash in seen_hashes:
            print(f"❌ ДУБЛИРОВАНИЕ ХЭША!")
            print(f"   Блок {i} имеет хэш, который уже встречался ранее")
            print(f"   Хэш: {current_hash[:32] if len(current_hash) > 32 else current_hash}...")
            errors += 1
        else:
            seen_hashes.add(current_hash)
        
        # Получаем prevHash из заголовка
        header = block.get("header", {})
        prev_hash = normalize_hash(header.get("prevHash"))
        
        # Для первого блока (genesis) prevHash может быть пустым или нулевым
        if i == 0:
            hash_short = current_hash[:16] + "..." if len(current_hash) > 16 else current_hash
            print(f"✅ Genesis блок (hash: {hash_short})")
            previous_hash = current_hash
            continue
        
        # Проверяем, что prevHash текущего блока равен hash предыдущего
        if prev_hash == "" or prev_hash == "0" or prev_hash == "0000000000000000000000000000000000000000000000000000000000000000":
            # Genesis блок имеет нулевой prevHash
            if i == 0:
                hash_short = current_hash[:16] + "..." if len(current_hash) > 16 else current_hash
                print(f"✅ Genesis блок (hash: {hash_short})")
                previous_hash = current_hash
                continue
        
        # Сравниваем хэши цепочки
        if prev_hash != previous_hash:
            print(f"❌ ОШИБКА целостности цепочки!")
            print(f"   Блок {i}: prevHash = {prev_hash[:32] if len(prev_hash) > 32 else prev_hash}...")
            print(f"   Блок {i-1}: hash = {previous_hash[:32] if len(previous_hash) > 32 else previous_hash}...")
            errors += 1
        else:
            # Обрезаем для красоты вывода
            hash_short = current_hash[:16] + "..." if len(current_hash) > 16 else current_hash
            print(f"✅ Hash: {hash_short}")
        
        previous_hash = current_hash
    
    print("=" * 60)
    print(f"📈 Статистика:")
    print(f"   Проверено блоков: {height}")
    print(f"   Уникальных хэшей: {len(seen_hashes)}")
    print(f"   Найдено ошибок: {errors}")
    print("=" * 60)
    
    if errors == 0:
        print(f"✅ Все блоки проверены: целостность цепочки подтверждена!")
        print(f"✅ Дублирование хэшей не обнаружено!")
        return True
    else:
        print(f"❌ Найдено ошибок: {errors}")
        return False

def main():
    """Главная функция"""
    api_url = "http://91.199.32.125:1337/app"
    
    # Можно передать URL как аргумент командной строки
    if len(sys.argv) > 1:
        api_url = sys.argv[1]
    
    success = check_blockchain_integrity(api_url)
    sys.exit(0 if success else 1)

if __name__ == "__main__":
    main()

