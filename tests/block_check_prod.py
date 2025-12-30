#!/usr/bin/env python3
"""
Скрипт для проверки целостности блокчейна:
- Получает высоту блокчейна
- Перебирает все блоки
- Проверяет, что хэши цепочки совпадают (prevHash == hash предыдущего блока)
- Проверяет, что хэши блоков не дублируются
- Проверяет, что сумма газа транзакций соответствует gasUsed в заголовке блока
- Проверяет, что nonce транзакций соответствует nonce в заголовке блока
"""

import requests
import json
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock

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
                # getInfo возвращает объект с полем total
                height = chain_info.get('total', 0)
                return int(height)
            else:
                # Если result - это число напрямую
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

def load_block_chunk(api_url: str, indices: list) -> dict:
    """Загружает chunk блоков параллельно"""
    results = {}
    for index in indices:
        block = get_block_by_index(api_url, index)
        if block:
            results[index] = block
    return results

def check_blockchain_integrity(api_url: str = "http://91.199.32.125:1337/app", 
                                num_threads: int = 10, 
                                chunk_size: int = 50) -> bool:
    """Проверяет целостность блокчейна многопоточно"""
    print("🔍 Проверка целостности блокчейна (многопоточная)")
    print(f"API URL: {api_url}")
    print(f"Потоков: {num_threads}, Размер chunk: {chunk_size}")
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
    print(f"📦 Загрузка {height} блоков в {num_threads} потоков...")
    
    # Разбиваем на chunks
    indices = list(range(height))
    chunks = [indices[i:i + chunk_size] for i in range(0, len(indices), chunk_size)]
    total_chunks = len(chunks)
    
    print(f"📦 Разбито на {total_chunks} chunk'ов по ~{chunk_size} блоков")
    print("=" * 60)
    
    # Загружаем блоки параллельно
    all_blocks = {}  # {index: block_data}
    errors = 0
    seen_hashes = set()
    seen_nonces = {}  # {nonce: first_block_index} - для проверки уникальности nonce
    lock = Lock()
    
    with ThreadPoolExecutor(max_workers=num_threads) as executor:
        # Создаем задачи для каждого chunk'а
        futures = {executor.submit(load_block_chunk, api_url, chunk): chunk for chunk in chunks}
        
        completed = 0
        for future in as_completed(futures):
            chunk = futures[future]
            try:
                chunk_results = future.result()
                all_blocks.update(chunk_results)
                completed += 1
                print(f"✅ Загружен chunk {completed}/{total_chunks} (блоки {chunk[0]}-{chunk[-1]})")
            except Exception as e:
                print(f"❌ Ошибка при загрузке chunk {chunk[0]}-{chunk[-1]}: {e}")
                errors += len(chunk)
    
    print("=" * 60)
    print(f"📦 Загружено блоков: {len(all_blocks)}/{height}")
    
    if len(all_blocks) < height:
        missing = height - len(all_blocks)
        print(f"⚠️  Пропущено блоков: {missing}")
        errors += missing
    
    print("🔍 Проверка целостности цепочки...")
    print("=" * 60)
    
    # Сортируем блоки по индексу для проверки цепочки
    sorted_indices = sorted(all_blocks.keys())
    previous_hash = None
    duplicate_errors = []
    duplicate_nonce_errors = []
    gas_mismatch_errors = []
    nonce_mismatch_errors = []
    
    for i in sorted_indices:
        block = all_blocks[i]
        current_hash = normalize_hash(block.get("hash"))
        
        # Проверка на дублирование хэшей
        with lock:
            if current_hash in seen_hashes:
                duplicate_errors.append((i, current_hash))
                errors += 1
            else:
                seen_hashes.add(current_hash)
        
        # Получаем заголовок блока
        header = block.get("header", {})
        block_nonce = header.get("nonce")
        gas_used = header.get("gasUsed")
        
        # Проверка уникальности nonce блока
        if block_nonce is not None:
            with lock:
                if block_nonce in seen_nonces:
                    # Находим, какой блок уже имел этот nonce
                    first_block_idx = seen_nonces[block_nonce]
                    duplicate_nonce_errors.append((i, block_nonce, first_block_idx))
                    errors += 1
                else:
                    seen_nonces[block_nonce] = i  # Сохраняем индекс первого блока с этим nonce
        
        # Проверка суммы газа по транзакциям
        transactions = block.get("transactions", [])
        if gas_used is not None:
            total_tx_gas = 0.0
            if transactions:
                for tx in transactions:
                    tx_gas = tx.get("gas")
                    if tx_gas is not None:
                        # Преобразуем в float, если это строка или число
                        try:
                            print(float(tx_gas))
                            total_tx_gas += float(tx_gas)
                        except (ValueError, TypeError):
                            pass
            
            # Сравниваем сумму газа транзакций с gasUsed в заголовке
            gas_used_float = float(gas_used) if gas_used is not None else 0.0
            # Допускаем небольшую погрешность из-за округления float
            if abs(total_tx_gas - gas_used_float) > 0.0001:
                gas_mismatch_errors.append((i, gas_used_float, total_tx_gas))
                errors += 1
        
        # Проверка сверки nonce в заголовке блока и в транзакциях
        if transactions and block_nonce is not None:
            for tx_idx, tx in enumerate(transactions):
                tx_nonce = tx.get("nonce")
                if tx_nonce is not None:
                    # Преобразуем в int для сравнения
                    try:
                        tx_nonce_int = int(tx_nonce)
                        block_nonce_int = int(block_nonce)
                        if tx_nonce_int != block_nonce_int:
                            nonce_mismatch_errors.append((i, tx_idx, block_nonce_int, tx_nonce_int))
                            errors += 1
                    except (ValueError, TypeError):
                        pass
        
        # Получаем prevHash из заголовка (header уже получен выше)
        prev_hash = normalize_hash(header.get("prevHash"))
        
        # Для первого блока (genesis) prevHash может быть пустым или нулевым
        if i == 0:
            hash_short = current_hash[:16] + "..." if len(current_hash) > 16 else current_hash
            print(f"✅ Блок 0: Genesis (hash: {hash_short})")
            previous_hash = current_hash
            continue
        
        # Проверяем, что prevHash текущего блока равен hash предыдущего
        zero_hash = "0000000000000000000000000000000000000000000000000000000000000000"
        if prev_hash and prev_hash != "" and prev_hash != "0" and prev_hash != zero_hash:
            if prev_hash != previous_hash:
                print(f"❌ ОШИБКА целостности цепочки!")
                print(f"   Блок {i}: prevHash = {prev_hash[:32]}...")
                print(f"   Блок {i-1}: hash = {previous_hash[:32] if previous_hash else 'N/A'}...")
                errors += 1
            else:
                if i % 100 == 0 or i == sorted_indices[-1]:
                    hash_short = current_hash[:16] + "..." if len(current_hash) > 16 else current_hash
                    print(f"✅ Блок {i}: Hash: {hash_short}")
        
        previous_hash = current_hash
    
    # Выводим информацию о дубликатах хэшей
    if duplicate_errors:
        print("=" * 60)
        print("❌ Найдены дублирующиеся хэши:")
        for block_idx, dup_hash in duplicate_errors:
            print(f"   Блок {block_idx}: хэш уже встречался ранее")
            print(f"   Хэш: {dup_hash[:32]}...")
    
    # Выводим информацию о дубликатах nonce
    if duplicate_nonce_errors:
        print("=" * 60)
        print("❌ Найдены дублирующиеся nonce:")
        for block_idx, dup_nonce, first_block_idx in duplicate_nonce_errors:
            print(f"   Блок {block_idx}: nonce = {dup_nonce} уже встречался в блоке {first_block_idx}")
    
    # Выводим информацию о несоответствии газа
    if gas_mismatch_errors:
        print("=" * 60)
        print("❌ Найдены несоответствия суммы газа:")
        for block_idx, header_gas_used, total_tx_gas in gas_mismatch_errors:
            print(f"   Блок {block_idx}: gasUsed в заголовке = {header_gas_used}, сумма газа транзакций = {total_tx_gas}")
            print(f"   Разница: {abs(header_gas_used - total_tx_gas):.6f}")
    
    # Выводим информацию о несоответствии nonce транзакций
    if nonce_mismatch_errors:
        print("=" * 60)
        print("❌ Найдены несоответствия nonce транзакций:")
        for block_idx, tx_idx, block_nonce, tx_nonce in nonce_mismatch_errors:
            print(f"   Блок {block_idx}, транзакция {tx_idx}: nonce блока = {block_nonce}, nonce транзакции = {tx_nonce}")
    
    print("=" * 60)
    print(f"📈 Статистика:")
    print(f"   Проверено блоков: {len(all_blocks)}")
    print(f"   Уникальных хэшей: {len(seen_hashes)}")
    print(f"   Уникальных nonce: {len(seen_nonces)}")
    print(f"   Ошибок дублирования хэшей: {len(duplicate_errors)}")
    print(f"   Ошибок дублирования nonce блоков: {len(duplicate_nonce_errors)}")
    print(f"   Ошибок несоответствия газа: {len(gas_mismatch_errors)}")
    print(f"   Ошибок несоответствия nonce транзакций: {len(nonce_mismatch_errors)}")
    print(f"   Всего найдено ошибок: {errors}")
    print("=" * 60)
    
    if errors == 0:
        print(f"✅ Все блоки проверены: целостность цепочки подтверждена!")
        print(f"✅ Дублирование хэшей не обнаружено!")
        print(f"✅ Дублирование nonce блоков не обнаружено!")
        print(f"✅ Сумма газа транзакций соответствует gasUsed в заголовках!")
        print(f"✅ Nonce транзакций соответствует nonce в заголовках блоков!")
        return True
    else:
        print(f"❌ Найдено ошибок: {errors}")
        if duplicate_errors:
            print(f"   - Дублирование хэшей: {len(duplicate_errors)}")
        if duplicate_nonce_errors:
            print(f"   - Дублирование nonce блоков: {len(duplicate_nonce_errors)}")
        if gas_mismatch_errors:
            print(f"   - Несоответствие газа: {len(gas_mismatch_errors)}")
        if nonce_mismatch_errors:
            print(f"   - Несоответствие nonce транзакций: {len(nonce_mismatch_errors)}")
        return False

def main():
    """Главная функция"""
    api_url = "http://localhost:1337/app"
    num_threads = 10
    chunk_size = 50
    
    # Парсинг аргументов командной строки
    if len(sys.argv) > 1:
        api_url = sys.argv[1]
    if len(sys.argv) > 2:
        try:
            num_threads = int(sys.argv[2])
        except ValueError:
            print(f"⚠️  Неверное значение потоков, используем по умолчанию: {num_threads}")
    if len(sys.argv) > 3:
        try:
            chunk_size = int(sys.argv[3])
        except ValueError:
            print(f"⚠️  Неверное значение chunk size, используем по умолчанию: {chunk_size}")
    
    success = check_blockchain_integrity(api_url, num_threads, chunk_size)
    sys.exit(0 if success else 1)

if __name__ == "__main__":
    main()

