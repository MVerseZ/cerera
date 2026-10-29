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
        normalized = parse_accounts_map(data)
        return json.dumps(normalized, sort_keys=True, separators=(",", ":"))
    except (TypeError, ValueError):
        return None


def normalize_account_key(key: Any) -> str:
    if isinstance(key, str):
        s = key.strip().lower()
        if s.startswith("0x"):
            return s
        return f"0x{s}"
    if isinstance(key, dict):
        for field in ("hex", "Hex", "address", "Address"):
            if field in key and key[field]:
                return normalize_account_key(str(key[field]))
    return str(key).strip().lower()


def parse_accounts_map(data: Optional[Dict[str, Any]]) -> Dict[str, float]:
    """Normalize getAll payload to address(hex) -> balance for set/diff comparisons."""
    if not isinstance(data, dict):
        return {}
    out: Dict[str, float] = {}
    for raw_key, raw_val in data.items():
        addr = normalize_account_key(raw_key)
        try:
            out[addr] = float(raw_val)
        except (TypeError, ValueError):
            continue
    return out


def _short_addr(addr: str, width: int = 14) -> str:
    if len(addr) <= width + 2:
        return addr
    return addr[: width + 2] + "…"


def print_accounts_diff_details(results: Dict[str, Dict[str, Any]]) -> None:
    """Explain how getAll maps differ (not JSON key order)."""
    maps: Dict[str, Dict[str, float]] = {
        node: parse_accounts_map(data.get("accounts"))
        for node, data in results.items()
    }
    nodes = [n for n, m in maps.items() if m]
    if len(nodes) < 2:
        return

    print("\n   Детали расхождения getAll (сравнение по адресу и балансу, порядок не важен):")

    counts = {n: len(maps[n]) for n in nodes}
    unique_counts = set(counts.values())
    print(f"   Число аккаунтов по нодам: {counts}")
    if len(unique_counts) == 1:
        n_acc = next(iter(unique_counts))
        print(
            f"   Количество записей одинаковое ({n_acc}), "
            "но наборы адресов или балансы не совпадают."
        )
    else:
        print(
            f"   Разное число записей (min={min(counts.values())}, "
            f"max={max(counts.values())})."
        )

    union_addrs = set().union(*(maps[n].keys() for n in nodes))
    common_addrs = set(maps[nodes[0]].keys())
    for n in nodes[1:]:
        common_addrs &= set(maps[n].keys())

    print(
        f"   Уникальных адресов в объединении: {len(union_addrs)}, "
        f"на всех нодах сразу: {len(common_addrs)}"
    )

    for node in nodes:
        others = set().union(*(set(maps[o].keys()) for o in nodes if o != node))
        exclusive = set(maps[node].keys()) - others
        if not exclusive:
            continue
        sample = sorted(exclusive)[:8]
        shown = ", ".join(_short_addr(a) for a in sample)
        extra = f" … (+{len(exclusive) - len(sample)})" if len(exclusive) > len(sample) else ""
        print(f"   Только на {node:8} ({len(exclusive)} адр.): {shown}{extra}")

    missing_rows: List[Tuple[str, List[str]]] = []
    for addr in sorted(union_addrs - common_addrs):
        present = [n for n in nodes if addr in maps[n]]
        absent = [n for n in nodes if addr not in maps[n]]
        if absent:
            missing_rows.append((addr, absent))
    if missing_rows:
        print(f"   Адреса не на всех нодах (показано до 12 из {len(missing_rows)}):")
        for addr, absent in missing_rows[:12]:
            present = [n for n in nodes if addr in maps[n]]
            print(
                f"      {_short_addr(addr, 18)}: есть {', '.join(present)}; "
                f"нет {', '.join(absent)}"
            )

    balance_mismatch: List[Tuple[str, Dict[str, float]]] = []
    for addr in sorted(common_addrs):
        bals = {n: maps[n][addr] for n in nodes}
        if len(set(bals.values())) > 1:
            balance_mismatch.append((addr, bals))

    if balance_mismatch:
        print(
            f"   Общие адреса с разными балансами "
            f"({len(balance_mismatch)} шт., показано до 10):"
        )
        for addr, bals in balance_mismatch[:10]:
            parts = ", ".join(f"{n}={bals[n]:g}" for n in nodes)
            print(f"      {_short_addr(addr, 18)}: {parts}")
    elif common_addrs:
        print("   У общих адресов балансы совпадают; расхождение — в составе (лишние/отсутствующие адреса).")


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


def head_state_root(block: Optional[Dict[str, Any]]) -> Optional[str]:
    if not block:
        return None
    header = block.get("header") or {}
    root = header.get("root") or header.get("stateRoot")
    return root if isinstance(root, str) and root else None


def print_state_root_comparison(results: Dict[str, Dict[str, Any]]) -> bool:
    print("\n" + "=" * 80)
    print("Сравнение stateRoot head-блоков")
    print("=" * 80)

    roots: List[str] = []
    for node_name, data in results.items():
        root = head_state_root(data.get("last_block"))
        if root is not None:
            roots.append(root)
            print(f"OK {node_name:8}: stateRoot = {root[:22]}...")
        else:
            print(f"-- {node_name:8}: stateRoot unavailable")

    state_roots_match = len(roots) >= 2 and len(set(roots)) == 1
    if state_roots_match:
        print("\nstateRoot совпадает на всех нодах — vault replay OK")
    elif len(roots) >= 2:
        print("\nstateRoot различается между нодами")
    return state_roots_match


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
            print("\nНаборы аккаунтов (getAll) различаются между нодами — live vault не совпал с chain replay.")
            print("   Подсказка: дождитесь финализации блоков; при старых volume сделайте docker compose down -v.")
            print("   Балансы выводятся из replay цепочки (+ реплицированный журнал faucet), не из P2P account snapshot.")
            by_fp: Dict[str, List[str]] = {}
            for node, fp in account_fps.items():
                if fp is None:
                    continue
                by_fp.setdefault(fp, []).append(node)
            for i, nodes in enumerate(by_fp.values(), 1):
                print(f"   Группа {i}: {', '.join(sorted(nodes))}")
            print_accounts_diff_details(results)
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


def get_mempool_info(api_url: str) -> Dict[str, Any] | None:
    result = make_jsonrpc_request(api_url, "cerera.pool.getInfo", [])
    if result and "error" not in result and "result" in result:
        return result["result"]
    return None


def mempool_tx_count(info: Dict[str, Any] | None) -> int:
    """cerera.pool.getInfo size (supports legacy PascalCase from older nodes)."""
    if not info:
        return 0
    if "size" in info:
        return int(info["size"])
    if "Size" in info:
        return int(info["Size"])
    return 0


def normalize_block_hash(hash_value: Any) -> str:
    if hash_value is None:
        return ""
    if isinstance(hash_value, str):
        return hash_value.replace("0x", "").lower()
    if isinstance(hash_value, dict):
        if "hex" in hash_value:
            return normalize_block_hash(hash_value["hex"])
        if "hash" in hash_value:
            return normalize_block_hash(hash_value["hash"])
    return str(hash_value).lower()


def check_chain_integrity(blocks_by_node: Dict[str, Dict[int, Dict]]) -> Tuple[bool, List[str]]:
    errors: List[str] = []

    min_height = None
    for blocks in blocks_by_node.values():
        if blocks:
            node_max = max(blocks.keys())
            if min_height is None or node_max < min_height:
                min_height = node_max

    if min_height is None:
        return True, []

    for index in range(min_height + 1):
        node_hashes: Dict[str, str] = {}
        for node_name, blocks in blocks_by_node.items():
            if index in blocks:
                node_hashes[node_name] = normalize_block_hash(blocks[index].get("hash"))

        if not node_hashes:
            continue

        if len(set(node_hashes.values())) > 1:
            errors.append(f"Block {index}: different hashes across nodes")
            for node, hash_val in node_hashes.items():
                errors.append(f"  {node}: {hash_val[:32]}...")

        if index > 0:
            for node_name, blocks in blocks_by_node.items():
                if index in blocks and (index - 1) in blocks:
                    current_block = blocks[index]
                    prev_block = blocks[index - 1]
                    prev_hash_from_header = normalize_block_hash(
                        current_block.get("header", {}).get("prevHash")
                    )
                    prev_hash_from_prev_block = normalize_block_hash(prev_block.get("hash"))
                    if (
                        prev_hash_from_header
                        and prev_hash_from_prev_block
                        and prev_hash_from_header != prev_hash_from_prev_block
                    ):
                        errors.append(
                            f"Block {index} ({node_name}): prevHash != previous block hash"
                        )

    for node_name, blocks in blocks_by_node.items():
        seen_hashes: Dict[str, int] = {}
        for index, block in blocks.items():
            block_hash = normalize_block_hash(block.get("hash"))
            if block_hash in seen_hashes:
                errors.append(
                    f"Duplicate hash on {node_name}: block {index} == block {seen_hashes[block_hash]}"
                )
            else:
                seen_hashes[block_hash] = index

    return len(errors) == 0, errors


def probe_nodes_full(
    node_names: List[str],
    ports: List[int],
    *,
    load_all_blocks: bool = False,
) -> Tuple[Dict[str, Dict[str, Any]], Dict[str, Dict[int, Dict]]]:
    """Query nodes; optionally load every block for integrity checks."""
    results: Dict[str, Dict[str, Any]] = {}
    blocks_by_node: Dict[str, Dict[int, Dict]] = {}

    for i, port in enumerate(ports):
        node_name = node_names[i]
        api_url = f"http://localhost:{port}/app"

        print(f"\nНода: {node_name} (порт {port})")
        print(f"   URL: {api_url}")

        accounts_map = get_all_accounts(api_url)
        if accounts_map is not None:
            print(
                f"   OK cerera.account.getAll: {len(accounts_map)} аккаунтов (id={RPC_ID_ACCOUNT_GET_ALL})"
            )
        else:
            print("   -- cerera.account.getAll: нет данных или ошибка")

        height = get_chain_height(api_url)
        if height is None:
            print("   ERR Ошибка получения высоты")
            results[node_name] = {
                "port": port,
                "url": api_url,
                "height": None,
                "last_block": None,
                "accounts": accounts_map,
                "mempool_info": None,
            }
            blocks_by_node[node_name] = {}
            continue

        print(f"   OK Высота блокчейна: {height}")

        if get_blockchain_info(api_url):
            print("   OK Информация о блокчейне получена")
        else:
            print("   -- Не удалось получить информацию о блокчейне")

        mempool_info = get_mempool_info(api_url)
        if mempool_info:
            print(f"   OK Мемпул: {mempool_tx_count(mempool_info)} транзакций")
        else:
            print("   -- Не удалось получить информацию о мемпуле")

        node_blocks: Dict[int, Dict] = {}
        last_block = None
        if height > 0:
            if load_all_blocks:
                print("   Загрузка блоков для проверки целостности...")
                for index in range(height):
                    block = get_block_by_index(api_url, index)
                    if block:
                        node_blocks[index] = block
                        if (index + 1) % 10 == 0 or (index + 1) == height:
                            print(f"      Загружено: {len(node_blocks)}/{height} блоков")
                    else:
                        print(f"      -- Не удалось загрузить блок {index}")
            else:
                last_block_index = height - 1
                block = get_block_by_index(api_url, last_block_index)
                if block:
                    node_blocks[last_block_index] = block

            if node_blocks:
                last_block = node_blocks[max(node_blocks.keys())]
                print(f"   OK Последний блок: {format_block_info(last_block)}")
        else:
            print("   -- Блокчейн пуст (высота = 0)")

        results[node_name] = {
            "port": port,
            "url": api_url,
            "height": height,
            "last_block": last_block,
            "accounts": accounts_map,
            "mempool_info": mempool_info,
            "blocks": node_blocks if load_all_blocks else None,
        }
        blocks_by_node[node_name] = node_blocks if load_all_blocks else {}

    return results, blocks_by_node


def run_cluster_smoke_check(
    compose_file: str,
    node_names: List[str],
    ports: List[int],
    *,
    chain_integrity: bool = False,
    subtitle: str = "",
) -> None:
    """Full smoke: heights, head hash, getAll (+ optional chain integrity walk)."""
    title = f"Проверка {len(node_names)} нод из deployments/{compose_file}"
    if subtitle:
        title = f"{title} {subtitle}"
    print("=" * 80)
    print(title)
    print("=" * 80)

    results, blocks_by_node = probe_nodes_full(
        node_names, ports, load_all_blocks=chain_integrity
    )

    print("\n" + "=" * 80)
    print("СВОДКА")
    print("=" * 80)
    print_height_summary(results)

    chain_heads_match = print_head_hash_comparison(results)
    accounts_ok = print_accounts_comparison(results)

    print("\n" + "=" * 80)
    print("ПРОВЕРКА МЕМПУЛА")
    print("=" * 80)
    for node_name, data in results.items():
        info = data.get("mempool_info")
        if info:
            print(f"OK {node_name:8}: {mempool_tx_count(info)} транзакций")
        else:
            print(f"-- {node_name:8}: недоступен")

    integrity_ok = True
    integrity_errors: List[str] = []
    if chain_integrity:
        print("\n" + "=" * 80)
        print("ПРОВЕРКА ЦЕЛОСТНОСТИ ЦЕПОЧКИ")
        print("=" * 80)
        valid_nodes = {node: blocks for node, blocks in blocks_by_node.items() if blocks}
        if not valid_nodes:
            print("-- Нет нод с загруженными блоками для проверки целостности")
        else:
            print(f"Проверка целостности между {len(valid_nodes)} нодами...")
            integrity_ok, integrity_errors = check_chain_integrity(valid_nodes)
            if integrity_ok:
                print("OK Целостность цепочки подтверждена")
            else:
                print("ERR Обнаружены проблемы с целостностью цепочки:")
                for error in integrity_errors[:20]:
                    print(f"   {error}")
                if len(integrity_errors) > 20:
                    print(f"   ... и еще {len(integrity_errors) - 20} ошибок")

    print("\n" + "=" * 80)
    has_success, heights_ok = evaluate_chain_sync(results)
    ok = has_success and heights_ok and chain_heads_match and accounts_ok
    if chain_integrity:
        ok = ok and integrity_ok

    if ok:
        parts = ["высота", "head hash", "getAll"]
        if chain_integrity:
            parts.append("целостность цепочки")
        print(f"OK Все проверки пройдены ({', '.join(parts)})")
        sys.exit(0)

    print("ERR Обнаружены проблемы")
    if not has_success:
        print("   - Не все ноды доступны")
    if not heights_ok:
        print("   - Высоты не совпадают")
    if not chain_heads_match:
        print("   - Head-блоки различаются")
    if chain_integrity and not integrity_ok:
        print("   - Проблемы с целостностью цепочки")
    if not accounts_ok:
        print("   - Наборы cerera.account.getAll различаются (см. детали выше)")
    sys.exit(1)


def run_basic_cluster_check(
    compose_file: str,
    node_names: List[str],
    ports: List[int],
    *,
    check_state_root: bool = False,
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
    state_roots_match = True
    if check_state_root:
        state_roots_match = print_state_root_comparison(results)
    accounts_match = print_accounts_comparison(results)
    print_node_details(results)

    print("\n" + "=" * 80)
    has_success, heights_ok = evaluate_chain_sync(results)
    ok = has_success and heights_ok and chain_heads_match and accounts_match
    if check_state_root:
        ok = ok and state_roots_match

    if ok:
        sys.exit(0)

    print("ERR Обнаружены проблемы")
    if not has_success:
        print("   - Не все ноды доступны")
    if not heights_ok:
        print("   - Высоты не совпадают")
    if not chain_heads_match:
        print("   - Head-блоки различаются")
    if check_state_root and not state_roots_match:
        print("   - State root различается")
    if not accounts_match:
        print("   - Наборы cerera.account.getAll различаются (см. детали выше)")
    sys.exit(1)
