#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Check every block and parent link in one Cerera node's canonical chain."""

from __future__ import annotations

import argparse
import sys
from typing import Any, Dict, Optional

from check_nodes_common import configure_utf8_stdout, make_jsonrpc_request


def rpc_result(url: str, method: str, params: list[Any], timeout: int) -> Any:
    response = make_jsonrpc_request(url, method, params, timeout=timeout)
    if response is None:
        raise RuntimeError(f"{method}: no response")
    if not isinstance(response, dict):
        raise RuntimeError(f"{method}: unexpected response format")
    if "error" in response:
        detail = response.get("error")
        message = response.get("message") or response.get("text")
        raise RuntimeError(f"{method}: {detail}{': ' + str(message) if message else ''}")
    if "result" not in response:
        raise RuntimeError(f"{method}: response has no result")
    return response["result"]


def normalize_hash(value: Any) -> str:
    if isinstance(value, dict):
        value = value.get("hex", value.get("hash"))
    if not isinstance(value, str):
        return ""
    value = value.strip().lower()
    if value.startswith("0x"):
        value = value[2:]
    return value


def check_chain(url: str, timeout: int) -> None:
    height = rpc_result(url, "cerera.chain.height", [], timeout)
    if isinstance(height, bool) or not isinstance(height, int) or height < 0:
        raise RuntimeError(f"invalid chain height: {height!r}")

    print(f"Проверка цепочки по {url}")
    print(f"Высота head: {height}; блоков ожидается: {height + 1}")

    previous_hash: Optional[str] = None
    head_hash = ""
    for index in range(height + 1):
        block = rpc_result(url, "cerera.chain.getBlockByIndex", [index], timeout)
        if not isinstance(block, dict):
            raise RuntimeError(f"block {index}: missing or malformed block")

        header = block.get("header")
        if not isinstance(header, dict):
            raise RuntimeError(f"block {index}: missing header")
        block_height = header.get("height")
        if block_height != index:
            raise RuntimeError(f"block {index}: header height is {block_height!r}")

        block_hash = normalize_hash(block.get("hash"))
        if not block_hash:
            raise RuntimeError(f"block {index}: missing hash")

        if index > 0:
            parent_hash = normalize_hash(header.get("prevHash"))
            if not parent_hash:
                raise RuntimeError(f"block {index}: missing prevHash")
            if parent_hash != previous_hash:
                raise RuntimeError(
                    f"block {index}: prevHash {parent_hash} does not match "
                    f"block {index - 1} hash {previous_hash}"
                )

        previous_hash = block_hash
        head_hash = block_hash
        if (index + 1) % 100 == 0 or index == height:
            print(f"Проверено блоков: {index + 1}/{height + 1}")

    print(f"OK Цепочка связна от генезиса до head {head_hash}")


def main() -> int:
    configure_utf8_stdout()
    parser = argparse.ArgumentParser(description="Проверить целостность цепочки одной ноды Cerera.")
    parser.add_argument(
        "--url",
        default="http://localhost:1337/app",
        help="JSON-RPC URL ноды (по умолчанию: http://localhost:1337/app)",
    )
    parser.add_argument("--timeout", type=int, default=10, help="таймаут одного RPC-запроса в секундах")
    args = parser.parse_args()

    if args.timeout <= 0:
        parser.error("--timeout должен быть положительным")

    try:
        check_chain(args.url, args.timeout)
        return 0
    except RuntimeError as exc:
        print(f"ERR Проверка цепочки не пройдена: {exc}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
