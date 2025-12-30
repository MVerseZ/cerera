#!/usr/bin/env python3
"""
Скрипт для проверки блоков в очереди:
- В цикле получает блоки
- Сравнивает каждый блок с предыдущим по хэшу (prevHash == hash предыдущего)
- Проверяет сумму газа транзакций каждого блока
"""

import requests
import json
import sys
from datetime import datetime


class Tee:
    """Класс для дублирования вывода в консоль и файл"""
    def __init__(self, *files):
        self.files = files
    
    def write(self, obj):
        for f in self.files:
            f.write(obj)
            f.flush()
    
    def flush(self):
        for f in self.files:
            f.flush()


def get_chain_height(api_url: str) -> int:
    """Получает высоту блокчейна"""
    data_req = {
        "method": "cerera.chain.getInfo",
        "jsonrpc": "2.0",
        "id": 1,
        "params": []
    }
    
    try:
        response = requests.post(api_url, json=data_req, timeout=10)
        if response.status_code == 200:
            result = response.json()
            chain_info = result.get('result', {})
            if isinstance(chain_info, dict):
                height = chain_info.get('total', 0)
                return int(height)
            else:
                return int(chain_info)
        else:
            print(f"❌ Ошибка получения высоты: {response.text}")
            return -1
    except Exception as e:
        print(f"❌ Исключение при получении высоты: {e}")
        return -1


def get_block_by_index(api_url: str, index: int) -> dict:
    """Получает блок по индексу"""
    data_req = {
        "method": "cerera.chain.getBlockByIndex",
        "jsonrpc": "2.0",
        "id": 2,
        "params": [index]
    }
    
    try:
        response = requests.post(api_url, json=data_req, timeout=10)
        if response.status_code == 200:
            result = response.json()
            block = result.get('result')
            return block if block else {}
        else:
            print(f"❌ Ошибка получения блока {index}: {response.text}")
            return {}
    except Exception as e:
        print(f"❌ Исключение при получении блока {index}: {e}")
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


def calculate_total_gas(transactions: list) -> float:
    """Вычисляет сумму газа всех транзакций в блоке"""
    total_gas = 0.0
    if transactions:
        for tx in transactions:
            tx_gas = tx.get("gas")
            if tx_gas is not None:
                try:
                    total_gas += float(tx_gas)
                except (ValueError, TypeError):
                    pass
    return total_gas


def check_block_queue(api_url: str = "http://localhost:1337/app", output_file: str = None) -> bool:
    """Проверяет блоки в очереди: сравнивает хэши и проверяет сумму газа"""
    # Настраиваем вывод в файл, если указан
    original_stdout = sys.stdout
    file_handle = None
    
    if output_file:
        try:
            file_handle = open(output_file, 'w', encoding='utf-8')
            # Перенаправляем вывод в консоль и файл
            sys.stdout = Tee(original_stdout, file_handle)
            print(f"📝 Вывод сохраняется в файл: {output_file}")
            print("=" * 60)
        except Exception as e:
            print(f"❌ Ошибка открытия файла {output_file}: {e}", file=original_stdout)
            file_handle = None
    
    try:
        print("🔍 Проверка блоков в очереди")
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
        print("=" * 60)
        
        previous_hash = None
        errors = 0
        
        # В цикле получаем блоки
        for i in range(height):
            block = get_block_by_index(api_url, i)
            
            if not block:
                print(f"❌ Не удалось получить блок {i}")
                errors += 1
                continue
            
            # Получаем хэш текущего блока
            current_hash = normalize_hash(block.get("hash"))
            header = block.get("header", {})
            prev_hash = normalize_hash(header.get("prevHash"))
            
            # Получаем транзакции
            transactions = block.get("transactions", [])
            
            # Вычисляем сумму газа транзакций
            total_tx_gas = calculate_total_gas(transactions)
            
            # Для первого блока (genesis) prevHash может быть пустым
            if i == 0:
                hash_short = current_hash[:16] + "..." if len(current_hash) > 16 else current_hash
                print(f"✅ Блок 0 (Genesis)")
                print(f"   Hash: {hash_short}")
                print(f"   Транзакций: {len(transactions)}")
                print(f"   Сумма газа транзакций: {total_tx_gas}")
                previous_hash = current_hash
                continue
            
            # Проверяем, что prevHash текущего блока равен hash предыдущего
            zero_hash = "0000000000000000000000000000000000000000000000000000000000000000"
            hash_match = True
            
            if prev_hash and prev_hash != "" and prev_hash != "0" and prev_hash != zero_hash:
                if prev_hash != previous_hash:
                    print(f"❌ ОШИБКА: Блок {i}")
                    print(f"   prevHash блока {i}: {prev_hash[:32]}...")
                    print(f"   hash блока {i-1}: {previous_hash[:32] if previous_hash else 'N/A'}...")
                    hash_match = False
                    errors += 1
            
            # Выводим информацию о блоке
            hash_short = current_hash[:16] + "..." if len(current_hash) > 16 else current_hash
            prev_hash_short = prev_hash[:16] + "..." if len(prev_hash) > 16 else prev_hash
            
            status = "✅" if hash_match else "❌"
            print(f"{status} Блок {i}")
            print(f"   Hash: {hash_short}")
            print(f"   PrevHash: {prev_hash_short}")
            print(f"   Транзакций: {len(transactions)}")
            print(f"   Сумма газа транзакций: {total_tx_gas}")
            
            if not hash_match:
                print(f"   ⚠️  Хэши не совпадают!")
            
            previous_hash = current_hash
        
        print("=" * 60)
        print(f"📈 Статистика:")
        print(f"   Проверено блоков: {height}")
        print(f"   Найдено ошибок: {errors}")
        print("=" * 60)
        
        if errors == 0:
            print(f"✅ Все блоки проверены: целостность цепочки подтверждена!")
            result = True
        else:
            print(f"❌ Найдено ошибок: {errors}")
            result = False
        
        return result
    finally:
        # Восстанавливаем стандартный вывод
        if file_handle:
            sys.stdout = original_stdout
            file_handle.close()
            print(f"✅ Результаты сохранены в файл: {output_file}", file=original_stdout)


def main():
    """Главная функция"""
    api_url = "http://localhost:1337/app"
    output_file = None
    
    # Парсинг аргументов командной строки
    if len(sys.argv) > 1:
        api_url = sys.argv[1]
    if len(sys.argv) > 2:
        output_file = sys.argv[2]
    else:
        # Если файл не указан, создаем автоматическое имя с датой/временем
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        output_file = f"block_queue_test_{timestamp}.log"
    
    success = check_block_queue(api_url, output_file)
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()

