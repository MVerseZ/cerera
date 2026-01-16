#!/usr/bin/env python3
"""
Скрипт для проверки первых 3 нод из docker-compose-15nodes.yml
Делает запросы на высоту блокчейна и последний блок для каждой ноды
"""

import requests
import json
import sys
from typing import Dict, Optional, List, Any
from datetime import datetime


# Порты первых 3 нод из docker-compose-15nodes.yml
DOCKER_COMPOSE_PORTS = [1337, 1338, 1339]  # node1, node2, node3
DOCKER_COMPOSE_NODES = ['node1', 'node2', 'node3']


def make_jsonrpc_request(api_url: str, method: str, params: List = None, timeout: int = 10) -> Optional[Dict]:
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
        "id": 1
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


def main():
    print("=" * 80)
    print("📡 Проверка первых 3 нод из docker-compose-15nodes.yml")
    print("   Ноды: node1 (1337), node2 (1338), node3 (1339)")
    print("   Запросы: высота блокчейна, информация о блокчейне и последний блок")
    print("=" * 80)
    print()
    
    results = {}
    
    # Делаем запросы на первые 3 ноды
    for i, port in enumerate(DOCKER_COMPOSE_PORTS):
        node_name = DOCKER_COMPOSE_NODES[i]
        api_url = f"http://localhost:{port}/app"
        
        print(f"\n🔍 Нода: {node_name} (порт {port})")
        print(f"   URL: {api_url}")
        
        # Получаем высоту
        height = get_chain_height(api_url)
        
        if height is None:
            print(f"   ❌ Ошибка получения высоты")
            results[node_name] = {
                "port": port,
                "url": api_url,
                "height": None,
                "blockchain_info": None,
                "last_block": None,
                "error": "Не удалось получить высоту"
            }
            continue
        
        print(f"   ✅ Высота блокчейна: {height}")
        
        # Получаем информацию о блокчейне
        blockchain_info = get_blockchain_info(api_url)
        if blockchain_info:
            print(f"   ✅ Информация о блокчейне получена")
        else:
            print(f"   ⚠️  Не удалось получить информацию о блокчейне")
        
        # Получаем последний блок (индекс = height - 1)
        last_block_index = height - 1 if height > 0 else 0
        last_block = None
        
        if height > 0:
            last_block = get_block_by_index(api_url, last_block_index)
            if last_block:
                print(f"   ✅ Последний блок (индекс {last_block_index}):")
                block_info = format_block_info(last_block)
                print(f"      {block_info}")
            else:
                print(f"   ⚠️  Не удалось получить последний блок (индекс {last_block_index})")
        else:
            print(f"   ℹ️  Блокчейн пуст (высота = 0)")
        
        results[node_name] = {
            "port": port,
            "url": api_url,
            "height": height,
            "blockchain_info": blockchain_info,
            "last_block_index": last_block_index,
            "last_block": last_block
        }
    
    # Сводка
    print("\n" + "=" * 80)
    print("📊 СВОДКА")
    print("=" * 80)
    
    successful = sum(1 for r in results.values() if r.get("height") is not None)
    failed = len(results) - successful
    
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
    
    # Детальная информация по каждой ноде
    print(f"\n📋 Детальная информация:")
    print("-" * 80)
    for node_name, data in results.items():
        status = "✅" if data.get("height") is not None else "❌"
        height = data.get("height", "N/A")
        port = data.get("port", "N/A")
        
        last_block_info = ""
        if data.get("last_block"):
            block = data["last_block"]
            header = block.get("header", {})
            block_hash = block.get("hash", "N/A")
            hash_short = block_hash[:16] + "..." if isinstance(block_hash, str) and len(block_hash) > 16 else block_hash
            txs = len(block.get("transactions", []))
            last_block_info = f" | Last block: {hash_short} ({txs} TXs)"
        
        print(f"{status} {node_name:8} (порт {port:4}): высота = {height}{last_block_info}")
    
    print("\n" + "=" * 80)
    
    # Проверяем, были ли успешные запросы
    has_success = any(r.get("height") is not None for r in results.values())
    sys.exit(0 if has_success else 1)


if __name__ == "__main__":
    main()

