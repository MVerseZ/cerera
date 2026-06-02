#!/usr/bin/env python3
"""
Скрипт для проверки 5 нод из docker-compose-5nodes.yml и целостности цепочки
Делает запросы на высоту блокчейна и блоки для каждой ноды,
проверяет синхронизацию между нодами и целостность цепочки
"""

import requests
import json
import sys
from typing import Dict, Optional, List, Any, Tuple
from datetime import datetime
from collections import defaultdict


# Порты всех 5 нод из docker-compose-5nodes.yml
DOCKER_COMPOSE_PORTS = [1337, 1338, 1339, 1340, 1341]  # node1-node5
DOCKER_COMPOSE_NODES = ['node1', 'node2', 'node3', 'node4', 'node5']

RPC_ID_ACCOUNT_GET_ALL = 11


def make_jsonrpc_request(
    api_url: str,
    method: str,
    params: List = None,
    timeout: int = 10,
    rpc_id: int = 1,
) -> Optional[Dict]:
    """
    Выполняет JSON-RPC запрос на указанный адрес
    
    Args:
        api_url: URL API
        method: Метод JSON-RPC
        params: Параметры метода
        timeout: Таймаут запроса в секундах
    
    Returns:
        dict: Ответ от API или None в случае ошибки
    """
    if params is None:
        params = []
    
    data = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": rpc_id,
    }
    
    try:
        response = requests.post(
            api_url,
            json=data,
            headers={'Content-Type': 'application/json'},
            timeout=timeout
        )
        
        if response.status_code == 200:
            return response.json()
        else:
            return {"error": f"HTTP {response.status_code}", "text": response.text}
            
    except requests.exceptions.ConnectionError as e:
        return {"error": "ConnectionError", "message": str(e)}
    except requests.exceptions.Timeout as e:
        return {"error": "Timeout", "message": str(e)}
    except Exception as e:
        return {"error": "Exception", "message": str(e)}


def get_chain_height(api_url: str) -> Optional[int]:
    """
    Получает высоту блокчейна
    
    Args:
        api_url: URL API
    
    Returns:
        int: Высота блокчейна или None в случае ошибки
    """
    result = make_jsonrpc_request(api_url, "cerera.chain.height", [])
    
    if result and "error" not in result:
        if "result" in result:
            return int(result["result"])
    return None


def get_block_by_index(api_url: str, index: int) -> Optional[Dict]:
    """
    Получает блок по индексу
    
    Args:
        api_url: URL API
        index: Индекс блока
    
    Returns:
        dict: Блок или None в случае ошибки
    """
    result = make_jsonrpc_request(api_url, "cerera.chain.getBlockByIndex", [index])
    
    if result and "error" not in result:
        if "result" in result:
            return result["result"]
    return None


def get_blockchain_info(api_url: str) -> Optional[Dict]:
    """
    Получает информацию о блокчейне
    
    Args:
        api_url: URL API
    
    Returns:
        dict: Информация о блокчейне или None в случае ошибки
    """
    result = make_jsonrpc_request(api_url, "cerera.chain.getInfo", [])
    
    if result and "error" not in result:
        if "result" in result:
            return result["result"]
    return None


def get_mempool_info(api_url: str) -> Optional[Dict]:
    """
    Получает информацию о мемпуле
    
    Args:
        api_url: URL API
    
    Returns:
        dict: Информация о мемпуле или None в случае ошибки
    """
    result = make_jsonrpc_request(api_url, "cerera.pool.getInfo", [])
    
    if result and "error" not in result:
        if "result" in result:
            return result["result"]
    return None


def get_all_accounts(api_url: str) -> Optional[Dict]:
    """cerera.account.getAll с id=11 (как tests/total.py)."""
    result = make_jsonrpc_request(
        api_url, "cerera.account.getAll", [], rpc_id=RPC_ID_ACCOUNT_GET_ALL
    )
    if result and "error" not in result and "result" in result:
        r = result["result"]
        return r if isinstance(r, dict) else None
    return None


def fingerprint_accounts_map(data: Optional[Dict]) -> Optional[str]:
    if not isinstance(data, dict):
        return None
    try:
        return json.dumps(data, sort_keys=True, separators=(",", ":"))
    except (TypeError, ValueError):
        return None


def normalize_hash(hash_value) -> str:
    """
    Нормализует хэш к строке (обрабатывает разные форматы)
    
    Args:
        hash_value: Хэш в любом формате
    
    Returns:
        str: Нормализованный хэш
    """
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


def format_block_info(block: Dict) -> str:
    """
    Форматирует информацию о блоке для вывода
    
    Args:
        block: Данные блока
    
    Returns:
        str: Отформатированная строка
    """
    if not block:
        return "N/A"
    
    header = block.get("header", {})
    height = header.get("height", "N/A")
    hash_value = block.get("hash", "N/A")
    hash_short = hash_value[:20] + "..." if isinstance(hash_value, str) and len(hash_value) > 20 else hash_value
    timestamp = header.get("timestamp", 0)
    txs_count = len(block.get("transactions", []))
    
    time_str = ""
    if timestamp:
        try:
            dt = datetime.fromtimestamp(timestamp / 1000)
            time_str = f" | Time: {dt.strftime('%Y-%m-%d %H:%M:%S')}"
        except:
            pass
    
    return f"Height: {height} | Hash: {hash_short} | TXs: {txs_count}{time_str}"


def check_chain_integrity(blocks_by_node: Dict[str, Dict[int, Dict]]) -> Tuple[bool, List[str]]:
    """
    Проверяет целостность цепочки между нодами
    
    Args:
        blocks_by_node: Словарь {node_name: {index: block_data}}
    
    Returns:
        tuple: (success: bool, errors: List[str])
    """
    errors = []
    
    # Находим минимальную высоту среди всех нод
    min_height = None
    for node_name, blocks in blocks_by_node.items():
        if blocks:
            node_max = max(blocks.keys())
            if min_height is None or node_max < min_height:
                min_height = node_max
    
    if min_height is None:
        return True, []
    
    # Проверяем, что все ноды имеют одинаковые блоки
    for index in range(min_height + 1):
        node_hashes = {}
        
        # Собираем хеши блока с этого индекса от всех нод
        for node_name, blocks in blocks_by_node.items():
            if index in blocks:
                block = blocks[index]
                block_hash = normalize_hash(block.get("hash"))
                node_hashes[node_name] = block_hash
        
        if not node_hashes:
            continue
        
        # Проверяем, что все ноды имеют одинаковый хеш для этого блока
        unique_hashes = set(node_hashes.values())
        if len(unique_hashes) > 1:
            errors.append(f"Блок {index}: разные хеши между нодами")
            for node, hash_val in node_hashes.items():
                errors.append(f"  {node}: {hash_val[:32]}...")
        
        # Проверяем целостность цепочки (prevHash == hash предыдущего блока)
        if index > 0:
            prev_hashes = {}
            for node_name, blocks in blocks_by_node.items():
                if index in blocks and (index - 1) in blocks:
                    current_block = blocks[index]
                    prev_block = blocks[index - 1]
                    
                    prev_hash_from_header = normalize_hash(current_block.get("header", {}).get("prevHash"))
                    prev_hash_from_prev_block = normalize_hash(prev_block.get("hash"))
                    
                    if prev_hash_from_header and prev_hash_from_prev_block:
                        if prev_hash_from_header != prev_hash_from_prev_block:
                            errors.append(
                                f"Блок {index} ({node_name}): prevHash не совпадает с hash предыдущего блока"
                            )
                            errors.append(f"  prevHash: {prev_hash_from_header[:32]}...")
                            errors.append(f"  prev block hash: {prev_hash_from_prev_block[:32]}...")
    
    # Проверяем на дублирующиеся хеши в каждой ноде
    for node_name, blocks in blocks_by_node.items():
        seen_hashes = {}
        for index, block in blocks.items():
            block_hash = normalize_hash(block.get("hash"))
            if block_hash in seen_hashes:
                errors.append(
                    f"Дублирующийся хеш в {node_name}: блок {index} имеет тот же хеш, что и блок {seen_hashes[block_hash]}"
                )
            else:
                seen_hashes[block_hash] = index
    
    return len(errors) == 0, errors


def main():
    print("=" * 80)
    print("📡 Проверка 5 нод из docker-compose-5nodes.yml и целостности цепочки")
    print("   Ноды: node1 (1337), node2 (1338), node3 (1339), node4 (1340), node5 (1341)")
    print("   Проверка: высота, мемпул, getAll (id=11), синхронизация, целостность цепочки")
    print("=" * 80)
    print()
    
    results = {}
    blocks_by_node = {}  # {node_name: {index: block_data}}
    
    # Делаем запросы на все 5 нод
    for i, port in enumerate(DOCKER_COMPOSE_PORTS):
        node_name = DOCKER_COMPOSE_NODES[i]
        api_url = f"http://localhost:{port}/app"
        
        print(f"\n🔍 Нода: {node_name} (порт {port})")
        print(f"   URL: {api_url}")

        accounts_map = get_all_accounts(api_url)
        if accounts_map is not None:
            print(f"   ✅ cerera.account.getAll: {len(accounts_map)} аккаунтов (id={RPC_ID_ACCOUNT_GET_ALL})")
        else:
            print(f"   ⚠️  cerera.account.getAll: нет данных или ошибка (id={RPC_ID_ACCOUNT_GET_ALL})")
        
        # Получаем высоту
        height = get_chain_height(api_url)
        
        if height is None:
            print(f"   ❌ Ошибка получения высоты")
            results[node_name] = {
                "port": port,
                "url": api_url,
                "height": None,
                "blockchain_info": None,
                "blocks": {},
                "accounts": accounts_map,
                "error": "Не удалось получить высоту"
            }
            blocks_by_node[node_name] = {}
            continue
        
        print(f"   ✅ Высота блокчейна: {height}")
        
        # Получаем информацию о блокчейне
        blockchain_info = get_blockchain_info(api_url)
        if blockchain_info:
            print(f"   ✅ Информация о блокчейне получена")
        else:
            print(f"   ⚠️  Не удалось получить информацию о блокчейне")
        
        # Получаем информацию о мемпуле
        mempool_info = get_mempool_info(api_url)
        if mempool_info:
            mempool_size = mempool_info.get("size", 0)
            print(f"   ✅ Мемпул: {mempool_size} транзакций")
        else:
            print(f"   ⚠️  Не удалось получить информацию о мемпуле")
        
        # Загружаем все блоки для проверки целостности
        node_blocks = {}
        if height > 0:
            print(f"   📦 Загрузка блоков для проверки целостности...")
            loaded = 0
            for index in range(height):
                block = get_block_by_index(api_url, index)
                if block:
                    node_blocks[index] = block
                    loaded += 1
                    if (index + 1) % 10 == 0 or (index + 1) == height:
                        print(f"      Загружено: {loaded}/{height} блоков")
                else:
                    print(f"      ⚠️  Не удалось загрузить блок {index}")
            
            if node_blocks:
                last_block_index = max(node_blocks.keys())
                last_block = node_blocks[last_block_index]
                print(f"   ✅ Последний блок (индекс {last_block_index}):")
                block_info = format_block_info(last_block)
                print(f"      {block_info}")
        else:
            print(f"   ℹ️  Блокчейн пуст (высота = 0)")
        
        results[node_name] = {
            "port": port,
            "url": api_url,
            "height": height,
            "blockchain_info": blockchain_info,
            "mempool_info": mempool_info,
            "blocks": node_blocks,
            "accounts": accounts_map,
        }
        blocks_by_node[node_name] = node_blocks
    
    # Сводка по высотам
    print("\n" + "=" * 80)
    print("📊 СВОДКА ПО ВЫСОТАМ")
    print("=" * 80)
    
    successful = sum(1 for r in results.values() if r.get("height") is not None)
    failed = len(results) - successful
    heights: List[int] = []
    
    print(f"✅ Успешных запросов: {successful}/{len(results)}")
    print(f"❌ Ошибок: {failed}/{len(results)}")
    
    if successful > 0:
        heights = [r["height"] for r in results.values() if r.get("height") is not None]
        if heights:
            min_height = min(heights)
            max_height = max(heights)
            avg_height = sum(heights) / len(heights)
            
            print(f"\n📈 Статистика по высоте:")
            print(f"   Минимальная: {min_height}")
            print(f"   Максимальная: {max_height}")
            print(f"   Средняя: {avg_height:.2f}")
            
            if min_height != max_height:
                print(f"\n⚠️  ВНИМАНИЕ: Высоты различаются между нодами!")
                print(f"   Разница: {max_height - min_height} блоков")
                
                # Показываем ноды с разными высотами
                height_groups = {}
                for node, data in results.items():
                    if data.get("height") is not None:
                        h = data["height"]
                        if h not in height_groups:
                            height_groups[h] = []
                        height_groups[h].append(node)
                
                print(f"\n   Группы по высоте:")
                for h in sorted(height_groups.keys()):
                    nodes = height_groups[h]
                    print(f"      Высота {h}: {', '.join(nodes)}")
            else:
                print(f"\n✅ Все ноды имеют одинаковую высоту: {min_height}")

    print("\n" + "=" * 80)
    print("👛 СРАВНЕНИЕ cerera.account.getAll (id=11)")
    print("=" * 80)
    account_fps: Dict[str, Optional[str]] = {}
    for node_name, data in results.items():
        acc = data.get("accounts")
        fp = fingerprint_accounts_map(acc)
        account_fps[node_name] = fp
        if fp is not None:
            print(f"✅ {node_name:8}: аккаунтов = {len(acc)}")
        else:
            print(f"❌ {node_name:8}: getAll недоступен или ошибка")
    valid_acc_fps = [fp for fp in account_fps.values() if fp is not None]
    accounts_match = True
    if len(valid_acc_fps) >= 2:
        unique_acc = set(valid_acc_fps)
        if len(unique_acc) != 1:
            accounts_match = False
            print("\n⚠️  Наборы аккаунтов (getAll) различаются между нодами!")
            by_fp: Dict[str, List[str]] = {}
            for node, fp in account_fps.items():
                if fp is None:
                    continue
                by_fp.setdefault(fp, []).append(node)
            for i, nodes in enumerate(by_fp.values(), 1):
                print(f"   Группа {i}: {', '.join(sorted(nodes))}")
        else:
            print("\n✅ Снимок getAll совпадает на всех нодах, где ответ получен")
    elif len(valid_acc_fps) == 1:
        print("\nℹ️  Только одна нода вернула getAll — сравнение между нодами невозможно")
    else:
        print("\n⚠️  Ни одна нода не вернула getAll")
    
    # Детальная информация по каждой ноде
    print(f"\n📋 Детальная информация:")
    print("-" * 80)
    for node_name, data in results.items():
        status = "✅" if data.get("height") is not None else "❌"
        height = data.get("height", "N/A")
        port = data.get("port", "N/A")
        blocks_count = len(data.get("blocks", {}))
        
        last_block_info = ""
        if data.get("blocks"):
            blocks = data["blocks"]
            if blocks:
                last_index = max(blocks.keys())
                last_block = blocks[last_index]
                block_hash = last_block.get("hash", "N/A")
                hash_short = block_hash[:16] + "..." if isinstance(block_hash, str) and len(block_hash) > 16 else block_hash
                txs = len(last_block.get("transactions", []))
                last_block_info = f" | Last block: {hash_short} ({txs} TXs) | Загружено блоков: {blocks_count}"
        ac = data.get("accounts")
        ac_n = len(ac) if isinstance(ac, dict) else "—"
        print(f"{status} {node_name:8} (порт {port:4}): высота = {height} | getAll: {ac_n} acc{last_block_info}")
    
    # Проверка мемпула
    print("\n" + "=" * 80)
    print("💾 ПРОВЕРКА МЕМПУЛА")
    print("=" * 80)
    
    mempool_sizes = {}
    for node_name, data in results.items():
        if data.get("mempool_info"):
            size = data["mempool_info"].get("size", 0)
            mempool_sizes[node_name] = size
            print(f"✅ {node_name:8}: {size} транзакций")
        else:
            mempool_sizes[node_name] = None
            print(f"❌ {node_name:8}: недоступен")
    
    if mempool_sizes:
        valid_sizes = [s for s in mempool_sizes.values() if s is not None]
        if valid_sizes:
            min_size = min(valid_sizes)
            max_size = max(valid_sizes)
            avg_size = sum(valid_sizes) / len(valid_sizes)
            print(f"\n📊 Статистика мемпула:")
            print(f"   Минимум: {min_size}")
            print(f"   Максимум: {max_size}")
            print(f"   Среднее: {avg_size:.1f}")
    
    # Проверка целостности цепочки
    print("\n" + "=" * 80)
    print("🔗 ПРОВЕРКА ЦЕЛОСТНОСТИ ЦЕПОЧКИ")
    print("=" * 80)
    
    # Фильтруем ноды, у которых есть блоки
    valid_nodes = {node: blocks for node, blocks in blocks_by_node.items() if blocks}
    
    integrity_ok = True
    integrity_errors: List[str] = []
    if not valid_nodes:
        print("⚠️  Нет нод с загруженными блоками для проверки целостности")
    else:
        print(f"Проверка целостности между {len(valid_nodes)} нодами...")
        integrity_ok, integrity_errors = check_chain_integrity(valid_nodes)
        
        if integrity_ok:
            print("✅ Целостность цепочки подтверждена!")
            print("   - Все ноды имеют одинаковые блоки")
            print("   - Цепочка блоков целостна (prevHash == hash предыдущего блока)")
            print("   - Дублирующиеся хеши не обнаружены")
        else:
            print("❌ Обнаружены проблемы с целостностью цепочки:")
            for error in integrity_errors[:20]:  # Показываем первые 20 ошибок
                print(f"   {error}")
            if len(integrity_errors) > 20:
                print(f"   ... и еще {len(integrity_errors) - 20} ошибок")
    
    print("\n" + "=" * 80)
    
    # Итоговый результат
    has_success = any(r.get("height") is not None for r in results.values())
    all_synced = successful == len(results) and all(
        r.get("height") == heights[0] for r in results.values() if r.get("height") is not None
    ) if heights else False
    mempool_available = all(r.get("mempool_info") is not None for r in results.values())
    
    if has_success and all_synced and integrity_ok and accounts_match:
        print("✅ ВСЕ ПРОВЕРКИ ПРОЙДЕНЫ УСПЕШНО")
        print("   - Все ноды доступны")
        print("   - Все ноды синхронизированы")
        print("   - Целостность цепочки подтверждена")
        print("   - cerera.account.getAll (id=11) совпадает между нодами")
        if mempool_available:
            print("   - Мемпул доступен на всех нодах")
        else:
            print("   - ⚠️  Мемпул недоступен на некоторых нодах")
        sys.exit(0)
    else:
        print("❌ ОБНАРУЖЕНЫ ПРОБЛЕМЫ")
        if not has_success:
            print("   - Не все ноды доступны")
        if not all_synced:
            print("   - Ноды не синхронизированы")
        if not integrity_ok:
            print("   - Проблемы с целостностью цепочки")
        if not accounts_match:
            print("   - Различие cerera.account.getAll между нодами")
        if not mempool_available:
            print("   - Мемпул недоступен на некоторых нодах")
        sys.exit(1)


if __name__ == "__main__":
    main()
