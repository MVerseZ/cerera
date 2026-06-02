#!/usr/bin/env python3
"""
Проверка создания аккаунтов Cerera через RPC: create, getBalance, getAll, verify.

Запуск:
    python accounts_test.py
    python accounts_test.py -c 5 --url http://localhost:1337/app
    python accounts_test.py -c 10 -v
"""

from __future__ import annotations

import argparse
import random
import secrets
import sys
from typing import Any, Dict, List, Optional

import requests

DEFAULT_API_URL = "http://localhost:1337/app"
DEFAULT_COUNT = 3


def jsonrpc_call(
    api_url: str,
    method: str,
    params: List[Any],
    rpc_id: Optional[int] = None,
    timeout: int = 30,
) -> Optional[Dict[str, Any]]:
    """JSON-RPC POST; при ошибке печатает сообщение и возвращает None."""
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": rpc_id if rpc_id is not None else random.randint(1, 1_000_000),
    }
    try:
        r = requests.post(
            api_url,
            json=payload,
            headers={"Content-Type": "application/json"},
            timeout=timeout,
        )
        if r.status_code != 200:
            print(f"❌ HTTP {r.status_code}: {method}\n   {r.text[:500]}")
            return None
        return r.json()
    except requests.exceptions.RequestException as e:
        print(f"❌ Ошибка запроса {method}: {e}")
        return None


def create_account(api_url: str, passphrase: str) -> Optional[Dict[str, Any]]:
    data = jsonrpc_call(api_url, "cerera.account.create", [passphrase])
    if not data:
        return None
    if "error" in data:
        print(f"❌ cerera.account.create: {data['error']}")
        return None
    res = data.get("result")
    if not res:
        print(f"❌ cerera.account.create: пустой result: {data}")
        return None
    return res


def get_balance(api_url: str, address: str) -> Any:
    data = jsonrpc_call(api_url, "cerera.account.getBalance", [address])
    if not data or "error" in data:
        return None
    return data.get("result")


def get_all_accounts(api_url: str) -> Optional[Dict[str, Any]]:
    data = jsonrpc_call(api_url, "cerera.account.getAll", [], rpc_id=11)
    if not data or "error" in data:
        return None
    result = data.get("result")
    if not isinstance(result, dict):
        print(f"❌ cerera.account.getAll: ожидался dict, получено {type(result)}")
        return None
    return result


def verify_account(api_url: str, address: str, passphrase: str) -> Optional[bool]:
    data = jsonrpc_call(api_url, "cerera.account.verify", [address, passphrase])
    if not data or "error" in data:
        return None
    return bool(data.get("result"))


def addr_in_get_all_map(address: str, accounts: Dict[str, Any]) -> bool:
    if address in accounts:
        return True
    lower = address.lower()
    for k in accounts:
        if isinstance(k, str) and k.lower() == lower:
            return True
    return False


def check_chain_reachable(api_url: str) -> bool:
    data = jsonrpc_call(api_url, "cerera.chain.getInfo", [], rpc_id=1, timeout=10)
    if not data:
        return False
    if "error" in data:
        print(f"❌ cerera.chain.getInfo: {data['error']}")
        return False
    print(f"✅ Нода доступна: {api_url}")
    return True


def step_wallet_fields(api_url: str, verbose: bool) -> bool:
    """Один аккаунт: поля address, priv, pub, mnemonic."""
    passphrase = f"acc_test_fields_{secrets.token_hex(8)}"
    acc = create_account(api_url, passphrase)
    if not acc:
        return False
    for key in ("address", "priv", "pub", "mnemonic"):
        if key not in acc or not acc[key]:
            print(f"❌ Нет поля {key!r} в ответе create")
            return False
    if not str(acc["address"]).startswith("0x"):
        print(f"❌ address без префикса 0x: {acc['address']!r}")
        return False
    if verbose:
        print(f"   address: {acc['address']}")
    print("✅ Создание аккаунта: поля address / priv / pub / mnemonic на месте")
    return True


def step_batch_create_and_verify(
    api_url: str, count: int, verbose: bool
) -> bool:
    """Несколько аккаунтов: getBalance, getAll, verify."""
    print(f"🔧 Создание и проверка {count} аккаунтов...")
    created: List[Dict[str, Any]] = []
    for i in range(count):
        passphrase = f"acc_test_{i}_{secrets.token_hex(6)}"
        acc = create_account(api_url, passphrase)
        if not acc:
            return False
        created.append({"acc": acc, "passphrase": passphrase})
        if verbose:
            print(f"   [{i + 1}/{count}] address={acc['address']}")

    accounts_map = get_all_accounts(api_url)
    if accounts_map is None:
        return False
    print(f"   cerera.account.getAll: записей в карте: {len(accounts_map)}")

    for i, item in enumerate(created):
        acc = item["acc"]
        addr = acc["address"]
        phrase = item["passphrase"]

        bal = get_balance(api_url, addr)
        if not isinstance(bal, (int, float)):
            print(f"❌ getBalance для {addr!r}: ожидалось число, получено {bal!r}")
            return False

        if not addr_in_get_all_map(addr, accounts_map):
            print(f"❌ Адрес не найден в getAll: {addr!r}")
            return False

        v = verify_account(api_url, addr, phrase)
        if v is not True:
            print(f"❌ verify для {addr!r}: ожидалось True, получено {v!r}")
            return False

        short = f"{addr[:10]}...{addr[-6:]}" if len(addr) > 20 else addr
        if verbose:
            print(
                f"   проверка [{i + 1}/{count}] {short}  "
                f"balance={bal}  getAll=ok  verify=ok"
            )

    print(f"✅ Пакет: {count} аккаунтов — getBalance, getAll, verify")
    return True


def step_wrong_passphrase(api_url: str, verbose: bool) -> bool:
    passphrase = f"acc_test_v_{secrets.token_hex(8)}"
    acc = create_account(api_url, passphrase)
    if not acc:
        return False
    addr = acc["address"]
    ok = verify_account(api_url, addr, passphrase)
    if ok is not True:
        print(f"❌ verify с верным паролем: ожидалось True, получено {ok!r}")
        return False
    bad = verify_account(api_url, addr, passphrase + "_wrong")
    if bad is not False:
        print(f"❌ verify с неверным паролем: ожидалось False, получено {bad!r}")
        return False
    if verbose:
        print(f"   address: {addr}")
    print("✅ verify: верный пароль → True, неверный → False")
    return True


def run_accounts_test(
    api_url: str, count: int, verbose: bool
) -> bool:
    """Все шаги подряд; при первой ошибке возвращает False."""
    print("=" * 60)
    print("🧪 Тест аккаунтов Cerera (RPC)")
    print(f"   URL: {api_url}")
    print(f"   Количество в пакете: {count}")
    print("=" * 60)

    if not check_chain_reachable(api_url):
        return False

    if not step_wallet_fields(api_url, verbose):
        return False
    if not step_batch_create_and_verify(api_url, count, verbose):
        return False
    if not step_wrong_passphrase(api_url, verbose):
        return False

    print("=" * 60)
    print("🎉 Все проверки пройдены")
    print("=" * 60)
    return True


def _utf8_stdout() -> None:
    """Чтобы emoji и кириллица не ломали вывод в консоли Windows."""
    if hasattr(sys.stdout, "reconfigure"):
        try:
            sys.stdout.reconfigure(encoding="utf-8", errors="replace")
        except (OSError, ValueError, AttributeError):
            pass


def main() -> None:
    _utf8_stdout()
    parser = argparse.ArgumentParser(
        description="Проверка cerera.account.create / getBalance / getAll / verify",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Примеры:
  %(prog)s
  %(prog)s -c 5 --url http://localhost:1337/app
  %(prog)s -c 10 -v
        """,
    )
    parser.add_argument(
        "-c",
        "--count",
        type=int,
        default=DEFAULT_COUNT,
        help=f"Сколько аккаунтов создать в пакетной проверке (по умолчанию: {DEFAULT_COUNT})",
    )
    parser.add_argument(
        "--url",
        type=str,
        default=DEFAULT_API_URL,
        help=f"URL RPC endpoint (по умолчанию: {DEFAULT_API_URL})",
    )
    parser.add_argument(
        "-v",
        "--verbose",
        action="store_true",
        help="Подробный вывод (адреса, шаги проверки)",
    )

    args = parser.parse_args()

    if args.count < 1:
        print("❌ Количество аккаунтов должно быть не меньше 1")
        sys.exit(1)

    ok = run_accounts_test(args.url, args.count, args.verbose)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
