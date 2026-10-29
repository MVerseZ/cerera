#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Smoke check for deployments/docker-compose-5nodes.yml with full chain integrity validation.
"""

import sys
from typing import Any, Dict, List, Tuple

from check_nodes_common import (
    RPC_ID_ACCOUNT_GET_ALL,
    configure_utf8_stdout,
    evaluate_chain_sync,
    format_block_info,
    get_all_accounts,
    get_block_by_index,
    get_blockchain_info,
    get_chain_height,
    make_jsonrpc_request,
    print_accounts_comparison,
    print_head_hash_comparison,
    print_height_summary,
)

COMPOSE_FILE = "docker-compose-5nodes.yml"
PORTS = [1337, 1338, 1339, 1340, 1341]
NODES = ["node1", "node2", "node3", "node4", "node5"]


def get_mempool_info(api_url: str) -> Dict[str, Any] | None:
    result = make_jsonrpc_request(api_url, "cerera.pool.getInfo", [])
    if result and "error" not in result and "result" in result:
        return result["result"]
    return None


def normalize_hash(hash_value: Any) -> str:
    if hash_value is None:
        return ""
    if isinstance(hash_value, str):
        return hash_value.replace("0x", "").lower()
    if isinstance(hash_value, dict):
        if "hex" in hash_value:
            return normalize_hash(hash_value["hex"])
        if "hash" in hash_value:
            return normalize_hash(hash_value["hash"])
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
                node_hashes[node_name] = normalize_hash(blocks[index].get("hash"))

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
                    prev_hash_from_header = normalize_hash(
                        current_block.get("header", {}).get("prevHash")
                    )
                    prev_hash_from_prev_block = normalize_hash(prev_block.get("hash"))
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
            block_hash = normalize_hash(block.get("hash"))
            if block_hash in seen_hashes:
                errors.append(
                    f"Duplicate hash on {node_name}: block {index} == block {seen_hashes[block_hash]}"
                )
            else:
                seen_hashes[block_hash] = index

    return len(errors) == 0, errors


def main() -> None:
    configure_utf8_stdout()

    print("=" * 80)
    print(f"Проверка 5 нод из deployments/{COMPOSE_FILE} и целостности цепочки")
    print("=" * 80)

    results: Dict[str, Dict[str, Any]] = {}
    blocks_by_node: Dict[str, Dict[int, Dict]] = {}

    for i, port in enumerate(PORTS):
        node_name = NODES[i]
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
            print(f"   OK Мемпул: {mempool_info.get('size', 0)} транзакций")
        else:
            print("   -- Не удалось получить информацию о мемпуле")

        node_blocks: Dict[int, Dict] = {}
        last_block = None
        if height > 0:
            print("   Загрузка блоков для проверки целостности...")
            for index in range(height):
                block = get_block_by_index(api_url, index)
                if block:
                    node_blocks[index] = block
                    if (index + 1) % 10 == 0 or (index + 1) == height:
                        print(f"      Загружено: {len(node_blocks)}/{height} блоков")
                else:
                    print(f"      -- Не удалось загрузить блок {index}")

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
            "blocks": node_blocks,
        }
        blocks_by_node[node_name] = node_blocks

    print("\n" + "=" * 80)
    print("СВОДКА")
    print("=" * 80)
    print_height_summary(results)

    chain_heads_match = print_head_hash_comparison(results)
    print_accounts_comparison(results)

    print("\n" + "=" * 80)
    print("ПРОВЕРКА МЕМПУЛА")
    print("=" * 80)
    for node_name, data in results.items():
        info = data.get("mempool_info")
        if info:
            print(f"OK {node_name:8}: {info.get('size', 0)} транзакций")
        else:
            print(f"-- {node_name:8}: недоступен")

    print("\n" + "=" * 80)
    print("ПРОВЕРКА ЦЕЛОСТНОСТИ ЦЕПОЧКИ")
    print("=" * 80)

    valid_nodes = {node: blocks for node, blocks in blocks_by_node.items() if blocks}
    integrity_ok = True
    integrity_errors: List[str] = []
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
    ok = has_success and heights_ok and chain_heads_match and integrity_ok

    if ok:
        print("OK Все проверки пройдены (высота, head hash, целостность цепочки)")
        sys.exit(0)

    print("ERR Обнаружены проблемы")
    if not has_success:
        print("   - Не все ноды доступны")
    if not heights_ok:
        print("   - Высоты не совпадают")
    if not chain_heads_match:
        print("   - Head-блоки различаются")
    if not integrity_ok:
        print("   - Проблемы с целостностью цепочки")
    sys.exit(1)


if __name__ == "__main__":
    main()
