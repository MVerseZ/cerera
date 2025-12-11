#!/usr/bin/env python3
"""
Скрипт для выполнения запроса cerera.chain.getInfo на все Docker Compose ноды
Делает запросы на все ноды: 1337, 1338, 1339, 1340, 1341
Сравнивает ответы и выводит различия
"""

import requests
import json
import sys
from typing import Dict, Optional, List, Any, Set


# Порты всех нод из docker-compose-nodes.yml
DOCKER_COMPOSE_PORTS = [1337, 1338, 1339, 1340, 1341]
DOCKER_COMPOSE_NODES = ['node1', 'node2', 'node3', 'node4', 'node5']


def get_blockchain_info(api_url: str, timeout: int = 10) -> Optional[Dict]:
    """
    Выполняет запрос cerera.chain.getInfo на указанный адрес
    
    Args:
        api_url: URL API
        timeout: Таймаут запроса в секундах
    
    Returns:
        dict: Ответ от API или None в случае ошибки
    """
    data = {
        "method": "cerera.chain.getInfo",
        "jsonrpc": "2.0",
        "id": 6525
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


def deep_compare(obj1: Any, obj2: Any, path: str = "") -> List[str]:
    """
    Рекурсивно сравнивает два объекта и возвращает список различий
    
    Args:
        obj1: Первый объект
        obj2: Второй объект
        path: Текущий путь в структуре (для отображения)
    
    Returns:
        list: Список строк с описанием различий
    """
    differences = []
    
    # Если типы разные
    if type(obj1) != type(obj2):
        differences.append(f"{path}: разные типы - {type(obj1).__name__} vs {type(obj2).__name__}")
        return differences
    
    # Если это словарь
    if isinstance(obj1, dict):
        all_keys = set(obj1.keys()) | set(obj2.keys())
        for key in all_keys:
            new_path = f"{path}.{key}" if path else key
            if key not in obj1:
                differences.append(f"{new_path}: отсутствует в первом объекте, значение во втором: {json.dumps(obj2[key])}")
            elif key not in obj2:
                differences.append(f"{new_path}: отсутствует во втором объекте, значение в первом: {json.dumps(obj1[key])}")
            else:
                differences.extend(deep_compare(obj1[key], obj2[key], new_path))
    
    # Если это список
    elif isinstance(obj1, list):
        if len(obj1) != len(obj2):
            differences.append(f"{path}: разная длина списков - {len(obj1)} vs {len(obj2)}")
        else:
            for i, (item1, item2) in enumerate(zip(obj1, obj2)):
                differences.extend(deep_compare(item1, item2, f"{path}[{i}]"))
    
    # Если это примитивные типы
    else:
        if obj1 != obj2:
            differences.append(f"{path}: {json.dumps(obj1)} != {json.dumps(obj2)}")
    
    return differences


def compare_all_results(results: Dict[str, Dict]) -> None:
    """
    Сравнивает результаты от всех нод и выводит различия
    
    Args:
        results: Словарь с результатами от каждой ноды
    """
    # Фильтруем только успешные ответы
    successful_results = {
        node: data["response"] 
        for node, data in results.items() 
        if data["response"] and "error" not in data["response"]
    }
    
    if len(successful_results) < 2:
        print("\n" + "=" * 80)
        print("⚠️  Недостаточно успешных ответов для сравнения")
        print("=" * 80)
        return
    
    print("\n" + "=" * 80)
    print("🔍 СРАВНЕНИЕ РЕЗУЛЬТАТОВ")
    print("=" * 80)
    
    nodes = list(successful_results.keys())
    
    # Сравниваем каждую пару нод
    for i in range(len(nodes)):
        for j in range(i + 1, len(nodes)):
            node1 = nodes[i]
            node2 = nodes[j]
            result1 = successful_results[node1]
            result2 = successful_results[node2]
            
            print(f"\n📊 Сравнение {node1} vs {node2}:")
            print("-" * 80)
            
            # Сравниваем result части
            if "result" in result1 and "result" in result2:
                differences = deep_compare(result1["result"], result2["result"], "result")
                if differences:
                    print(f"   ❌ Найдено различий: {len(differences)}")
                    for diff in differences:
                        print(f"      • {diff}")
                else:
                    print(f"   ✅ Результаты идентичны")
            elif "result" in result1:
                print(f"   ⚠️  {node1} имеет result, {node2} - нет")
            elif "result" in result2:
                print(f"   ⚠️  {node2} имеет result, {node1} - нет")
            else:
                print(f"   ⚠️  Оба ответа не имеют result")
            
            # Сравниваем полные ответы (если нужны детали)
            full_differences = deep_compare(result1, result2, "")
            if full_differences and len(full_differences) > len(differences):
                other_diffs = [d for d in full_differences if not d.startswith("result")]
                if other_diffs:
                    print(f"   📋 Другие различия (не в result):")
                    for diff in other_diffs[:5]:  # Показываем первые 5
                        print(f"      • {diff}")
                    if len(other_diffs) > 5:
                        print(f"      ... и еще {len(other_diffs) - 5} различий")


def main():
    print("=" * 80)
    print("📡 Запросы cerera.chain.getInfo на все Docker Compose ноды")
    print("=" * 80)
    print()
    
    results = {}
    
    # Делаем запросы на все ноды
    for i, port in enumerate(DOCKER_COMPOSE_PORTS):
        node_name = DOCKER_COMPOSE_NODES[i] if i < len(DOCKER_COMPOSE_NODES) else f"node{i+1}"
        api_url = f"http://localhost:{port}/app"
        
        print(f"\n🔍 Нода: {node_name} (порт {port})")
        print(f"   URL: {api_url}")
        
        result = get_blockchain_info(api_url)
        results[node_name] = {
            "port": port,
            "url": api_url,
            "response": result
        }
        
        if result:
            if "error" in result:
                print(f"   ❌ Ошибка: {result.get('error')} - {result.get('message', result.get('text', ''))}")
            else:
                print(f"   ✅ Статус: 200 OK")
                print(f"   📦 Полный ответ:")
                print(json.dumps(result, indent=6, ensure_ascii=False))
                
                if "result" in result:
                    if result["result"] == {} or not result["result"]:
                        print(f"   ⚠️  ВНИМАНИЕ: result пустой!")
                    else:
                        print(f"   📊 Result:")
                        print(json.dumps(result["result"], indent=6, ensure_ascii=False))
        else:
            print(f"   ❌ Нет ответа")
    
    # Сводка
    print("\n" + "=" * 80)
    print("📊 Сводка:")
    print("=" * 80)
    successful = sum(1 for r in results.values() if r["response"] and "error" not in r["response"])
    failed = len(results) - successful
    print(f"✅ Успешных: {successful}")
    print(f"❌ Ошибок: {failed}")
    
    # Сравнение результатов
    compare_all_results(results)
    
    print("\n" + "=" * 80)
    
    # Проверяем, были ли успешные запросы
    has_success = any(
        r["response"] and "error" not in r["response"] 
        for r in results.values()
    )
    sys.exit(0 if has_success else 1)


if __name__ == "__main__":
    main()

