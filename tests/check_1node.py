#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Smoke check for deployments/docker-compose-single.yml (one node)."""

import sys

from check_nodes_common import (
    configure_utf8_stdout,
    format_block_info,
    get_all_accounts,
    get_block_by_index,
    get_chain_height,
)

COMPOSE_FILE = "docker-compose-single.yml"
PORT = 1337
NODE = "node1"


def main() -> None:
    configure_utf8_stdout()

    print("=" * 80)
    print(f"Проверка 1 ноды из deployments/{COMPOSE_FILE}")
    print("=" * 80)

    api_url = f"http://localhost:{PORT}/app"
    print(f"\nНода: {NODE} (порт {PORT})")
    print(f"   URL: {api_url}")

    accounts = get_all_accounts(api_url)
    if accounts is not None:
        print(f"   OK cerera.account.getAll: {len(accounts)} аккаунтов")
    else:
        print("   -- cerera.account.getAll недоступен")

    height = get_chain_height(api_url)
    if height is None:
        print("   ERR не удалось получить высоту")
        sys.exit(1)

    print(f"   OK высота: {height}")

    if height > 0:
        block = get_block_by_index(api_url, height - 1)
        if block:
            print(f"   OK head block: {format_block_info(block)}")
        else:
            print("   -- head block недоступен")
            sys.exit(1)

    print("\nSingle-node smoke OK")
    sys.exit(0)


if __name__ == "__main__":
    main()
