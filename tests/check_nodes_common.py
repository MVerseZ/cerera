# -*- coding: utf-8 -*-
"""Shared helpers for multi-node Cerera smoke checks."""

from __future__ import annotations

import io
import json
import sys
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

import requests

RPC_ID_ACCOUNT_GET_ALL = 11


def configure_utf8_stdout() -> None:
    """Windows consoles may default to cp1251; keep output readable."""
    if hasattr(sys.stdout, "buffer"):
        sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
    if hasattr(sys.stderr, "buffer"):
        sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding="utf-8", errors="replace")


def make_jsonrpc_request(
    api_url: str,
    method: str,
    params: Optional[List[Any]] = None,
    timeout: int = 10,
    rpc_id: int = 1,
) -> Optional[Dict[str, Any]]:
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
            headers={"Content-Type": "application/json"},
            timeout=timeout,
        )
        if response.status_code == 200:
            return response.json()
        return {"error": f"HTTP {response.status_code}", "text": response.text}
    except requests.exceptions.ConnectionError as exc:
        return {"error": "ConnectionError", "message": str(exc)}
    except requests.exceptions.Timeout as exc:
        return {"error": "Timeout", "message": str(exc)}
    except Exception as exc:  # noqa: BLE001 - smoke script
        return {"error": "Exception", "message": str(exc)}


def get_chain_height(api_url: str) -> Optional[int]:
    result = make_jsonrpc_request(api_url, "cerera.chain.height", [])
    if result and "error" not in result and "result" in result:
        return int(result["result"])
    return None


def get_block_by_index(api_url: str, index: int) -> Optional[Dict[str, Any]]:
    result = make_jsonrpc_request(api_url, "cerera.chain.getBlockByIndex", [index])
    if result and "error" not in result and "result" in result:
        return result["result"]
    return None


def get_blockchain_info(api_url: str) -> Optional[Dict[str, Any]]:
    result = make_jsonrpc_request(api_url, "cerera.chain.getInfo", [])
    if result and "error" not in result and "result" in result:
        return result["result"]
    return None


def get_all_accounts(api_url: str) -> Optional[Dict[str, Any]]:
    result = make_jsonrpc_request(
        api_url, "cerera.account.getAll", [], rpc_id=RPC_ID_ACCOUNT_GET_ALL
    )
    if result and "error" not in result and "result" in result:
        payload = result["result"]
        return payload if isinstance(payload, dict) else None
    return None


def fingerprint_accounts_map(data: Optional[Dict[str, Any]]) -> Optional[str]:
    if not isinstance(data, dict):
        return None
    try:
        return json.dumps(data, sort_keys=True, separators=(",", ":"))
    except (TypeError, ValueError):
        return None


def format_block_info(block: Dict[str, Any]) -> str:
    if not block:
        return "N/A"

    header = block.get("header", {})
    height = header.get("height", "N/A")
    hash_value = block.get("hash", "N/A")
    hash_short = (
        hash_value[:20] + "..."
        if isinstance(hash_value, str) and len(hash_value) > 20
        else hash_value
    )
    timestamp = header.get("timestamp", 0)
    txs_count = len(block.get("transactions", []))

    time_str = ""
    if timestamp:
        try:
            dt = datetime.fromtimestamp(timestamp / 1000)
            time_str = f" | Time: {dt.strftime('%Y-%m-%d %H:%M:%S')}"
        except (TypeError, ValueError, OSError):
            pass

    return f"Height: {height} | Hash: {hash_short} | TXs: {txs_count}{time_str}"


def collect_heights(results: Dict[str, Dict[str, Any]]) -> List[int]:
    return [r["height"] for r in results.values() if r.get("height") is not None]


def heights_match(results: Dict[str, Dict[str, Any]]) -> bool:
    heights = collect_heights(results)
    return len(heights) >= 2 and len(set(heights)) == 1


def print_height_summary(results: Dict[str, Dict[str, Any]]) -> None:
    successful = sum(1 for r in results.values() if r.get("height") is not None)
    failed = len(results) - successful

    print(f"Успешных запросов: {successful}/{len(results)}")
    print(f"Ошибок: {failed}/{len(results)}")

    heights = collect_heights(results)
    if not heights:
        return

    min_height = min(heights)
    max_height = max(heights)
    avg_height = sum(heights) / len(heights)

    print(f"\nСтатистика по высоте:")
    print(f"   Минимальная: {min_height}")
    print(f"   Максимальная: {max_height}")
    print(f"   Средняя: {avg_height:.2f}")

    if min_height != max_height:
        print(f"\nВНИМАНИЕ: Высоты различаются между нодами!")
        print(f"   Разница: {max_height - min_height} блоков")

        height_groups: Dict[int, List[str]] = {}
        for node, data in results.items():
            if data.get("height") is not None:
                height_groups.setdefault(data["height"], []).append(node)

        print(f"\n   Группы по высоте:")
        for height in sorted(height_groups.keys()):
            print(f"      Высота {height}: {', '.join(height_groups[height])}")
    else:
        print(f"\nВсе ноды имеют одинаковую высоту: {min_height}")


def print_head_hash_comparison(results: Dict[str, Dict[str, Any]]) -> bool:
    print("\n" + "=" * 80)
    print("Сравнение head-блоков")
    print("=" * 80)

    head_hashes: List[str] = []
    for node_name, data in results.items():
        block = data.get("last_block")
        if block and isinstance(block.get("hash"), str):
            head_hashes.append(block["hash"])
            print(f"OK {node_name:8}: head = {block['hash'][:22]}...")
        else:
            print(f"-- {node_name:8}: head block unavailable")

    chain_heads_match = len(head_hashes) >= 2 and len(set(head_hashes)) == 1
    if chain_heads_match:
        print("\nHead-блок совпадает на всех нодах — chain sync OK")
    elif len(head_hashes) >= 2:
        print("\nHead-блоки различаются — chain рассинхронизирован")
    return chain_heads_match


def print_accounts_comparison(results: Dict[str, Dict[str, Any]]) -> bool:
    print("\n" + "=" * 80)
    print("Сравнение cerera.account.getAll (id=11)")
    print("=" * 80)

    account_fps: Dict[str, Optional[str]] = {}
    for node_name, data in results.items():
        acc = data.get("accounts")
        fp = fingerprint_accounts_map(acc)
        account_fps[node_name] = fp
        if fp is not None:
            print(f"OK {node_name:8}: аккаунтов = {len(acc)}")
        else:
            print(f"-- {node_name:8}: getAll недоступен или ошибка")

    valid_fps = [fp for fp in account_fps.values() if fp is not None]
    accounts_match = True
    if len(valid_fps) >= 2:
        unique_acc = set(valid_fps)
        if len(unique_acc) != 1:
            accounts_match = False
            print("\nНаборы аккаунтов (getAll) различаются между нодами (ожидаемо при разных node address).")
            by_fp: Dict[str, List[str]] = {}
            for node, fp in account_fps.items():
                if fp is None:
                    continue
                by_fp.setdefault(fp, []).append(node)
            for i, nodes in enumerate(by_fp.values(), 1):
                print(f"   Группа {i}: {', '.join(sorted(nodes))}")
        else:
            print("\nСнимок getAll совпадает на всех нодах, где ответ получен")
    elif len(valid_fps) == 1:
        print("\nТолько одна нода вернула getAll — сравнение между нодами невозможно")
    else:
        print("\nНи одна нода не вернула getAll")

    return accounts_match


def print_node_details(results: Dict[str, Dict[str, Any]]) -> None:
    print(f"\nДетальная информация:")
    print("-" * 80)
    for node_name, data in results.items():
        status = "OK" if data.get("height") is not None else "ERR"
        height = data.get("height", "N/A")
        port = data.get("port", "N/A")

        last_block_info = ""
        if data.get("last_block"):
            block = data["last_block"]
            block_hash = block.get("hash", "N/A")
            hash_short = (
                block_hash[:16] + "..."
                if isinstance(block_hash, str) and len(block_hash) > 16
                else block_hash
            )
            txs = len(block.get("transactions", []))
            last_block_info = f" | Last block: {hash_short} ({txs} TXs)"

        ac = data.get("accounts")
        ac_n = len(ac) if isinstance(ac, dict) else "—"
        print(f"{status} {node_name:8} (порт {port:4}): высота = {height} | getAll: {ac_n} acc{last_block_info}")


def evaluate_chain_sync(results: Dict[str, Dict[str, Any]]) -> Tuple[bool, bool]:
    """Returns (has_success, chain_sync_ok)."""
    has_success = any(r.get("height") is not None for r in results.values())
    chain_sync_ok = has_success and heights_match(results)
    return has_success, chain_sync_ok


def probe_nodes(
    node_names: List[str],
    ports: List[int],
) -> Dict[str, Dict[str, Any]]:
    """Query height, head block, and accounts on each node."""
    results: Dict[str, Dict[str, Any]] = {}

    for i, port in enumerate(ports):
        node_name = node_names[i]
        api_url = f"http://localhost:{port}/app"

        print(f"\nНода: {node_name} (порт {port})")
        print(f"   URL: {api_url}")

        accounts_map = get_all_accounts(api_url)
        if accounts_map is not None:
            print(f"   OK cerera.account.getAll: {len(accounts_map)} аккаунтов (id={RPC_ID_ACCOUNT_GET_ALL})")
        else:
            print(f"   -- cerera.account.getAll: нет данных или ошибка")

        height = get_chain_height(api_url)
        if height is None:
            print("   ERR Ошибка получения высоты")
            results[node_name] = {
                "port": port,
                "url": api_url,
                "height": None,
                "last_block": None,
                "accounts": accounts_map,
            }
            continue

        print(f"   OK Высота блокчейна: {height}")

        last_block = None
        if height > 0:
            last_block_index = height - 1
            last_block = get_block_by_index(api_url, last_block_index)
            if last_block:
                print(f"   OK Последний блок (индекс {last_block_index}):")
                print(f"      {format_block_info(last_block)}")
            else:
                print(f"   -- Не удалось получить последний блок (индекс {last_block_index})")
        else:
            print("   -- Блокчейн пуст (высота = 0)")

        results[node_name] = {
            "port": port,
            "url": api_url,
            "height": height,
            "last_block": last_block,
            "accounts": accounts_map,
        }

    return results


def run_basic_cluster_check(
    compose_file: str,
    node_names: List[str],
    ports: List[int],
) -> int:
    """Standard multi-node smoke: heights + head hash must match."""
    print("=" * 80)
    print(f"Проверка {len(node_names)} нод из deployments/{compose_file}")
    print("=" * 80)

    results = probe_nodes(node_names, ports)

    print("\n" + "=" * 80)
    print("СВОДКА")
    print("=" * 80)
    print_height_summary(results)

    chain_heads_match = print_head_hash_comparison(results)
    print_accounts_comparison(results)
    print_node_details(results)

    print("\n" + "=" * 80)
    has_success, heights_ok = evaluate_chain_sync(results)
    ok = has_success and heights_ok and chain_heads_match
    sys.exit(0 if ok else 1)
