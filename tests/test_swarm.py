#!/usr/bin/env python3
"""
Рой аккаунтов: создаёт count кошельков, пополняет каждый через faucet
и в бесконечном цикле каждые time секунд шлёт с каждого по одной
транзакции (10% текущего баланса) на 10 случайных адресов из списка.
"""

import argparse
import random
import sys
import time

import requests

API_URL = "http://localhost:1337/app"
TARGETS_PER_SENDER = 10
FAUCET_AMOUNT = 10
TRANSFER_SHARE = 0.10


def rpc(method, params, api_url=API_URL):
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": random.randint(1000, 9999),
    }
    r = requests.post(api_url, json=payload, timeout=10)
    if r.status_code != 200:
        raise RuntimeError(f"{method} HTTP {r.status_code}: {r.text}")
    body = r.json()
    if "error" in body:
        raise RuntimeError(f"{method}: {body['error']}")
    return body.get("result")


def create_account(name, api_url):
    result = rpc("cerera.account.create", [f"{name}_pass"], api_url)
    if not result or "address" not in result:
        raise RuntimeError(f"не удалось создать аккаунт {name}: {result}")
    return result


def get_balance(address, api_url):
    result = rpc("cerera.account.getBalance", [address], api_url)
    return float(result or 0)


def faucet(address, amount, api_url):
    return rpc("cerera.account.faucet", [address, amount], api_url)


def send_tx(sender, to_addr, amount, api_url, message=""):
    return rpc(
        "cerera.transaction.send",
        [sender["priv"], to_addr, amount, 21000, message],
        api_url,
    )


def setup_accounts(count, api_url):
    accounts = []
    print(f"Создание {count} аккаунтов...")
    for i in range(count):
        acc = create_account(f"swarm_{i}", api_url)
        accounts.append(acc)
        print(f"  [{i + 1}/{count}] {acc['address']}")

    print(f"\nFaucet {FAUCET_AMOUNT} на каждый аккаунт...")
    for i, acc in enumerate(accounts):
        result = faucet(acc["address"], FAUCET_AMOUNT, api_url)
        print(f"  [{i + 1}/{count}] {acc['address']}: {result}")

    time.sleep(0.5)
    print("\nБалансы после faucet:")
    for i, acc in enumerate(accounts):
        print(f"  [{i + 1}] {acc['address']}: {get_balance(acc['address'], api_url)}")
    return accounts


def pick_targets(sender, accounts):
    others = [acc for acc in accounts if acc["address"] != sender["address"]]
    n = min(TARGETS_PER_SENDER, len(others))
    return random.sample(others, n)


def swarm_round(accounts, api_url, round_no):
    sent = 0
    failed = 0
    print(f"\n--- раунд {round_no} ---")
    for sender in accounts:
        balance = get_balance(sender["address"], api_url)
        amount = balance * TRANSFER_SHARE
        if amount <= 0:
            print(f"  skip {sender['address'][:10]}... баланс {balance}")
            continue
        for target in pick_targets(sender, accounts):
            try:
                tx_hash = send_tx(
                    sender,
                    target["address"],
                    amount,
                    api_url,
                    message=f"swarm r{round_no}",
                )
                sent += 1
                print(
                    f"  {sender['address'][:10]}... -> {target['address'][:10]}... "
                    f"{amount:.6f}  {tx_hash}"
                )
            except Exception as e:
                failed += 1
                print(
                    f"  fail {sender['address'][:10]}... -> {target['address'][:10]}... "
                    f"{amount:.6f}: {e}"
                )
    print(f"раунд {round_no}: отправлено {sent}, ошибок {failed}")
    return sent, failed


def main():
    parser = argparse.ArgumentParser(
        description="Рой аккаунтов: faucet и бесконечные переводы 10% на случайные адреса",
    )
    parser.add_argument(
        "-c",
        "--count",
        type=int,
        required=True,
        help="Сколько аккаунтов создать",
    )
    parser.add_argument(
        "-t",
        "--time",
        type=float,
        required=True,
        help="Пауза между раундами в секундах",
    )
    parser.add_argument(
        "--url",
        type=str,
        default=API_URL,
        help=f"RPC endpoint (по умолчанию {API_URL})",
    )
    args = parser.parse_args()

    if args.count < 2:
        print("Нужно минимум 2 аккаунта", file=sys.stderr)
        sys.exit(1)
    if args.time < 0:
        print("Интервал не может быть отрицательным", file=sys.stderr)
        sys.exit(1)

    accounts = setup_accounts(args.count, args.url)
    targets = min(TARGETS_PER_SENDER, args.count - 1)
    print(
        f"\nЦикл: каждые {args.time} с каждый аккаунт шлёт "
        f"{targets} tx по {TRANSFER_SHARE:.0%} баланса. Ctrl+C для остановки."
    )

    round_no = 0
    try:
        while True:
            round_no += 1
            started = time.monotonic()
            swarm_round(accounts, args.url, round_no)
            wait = args.time - (time.monotonic() - started)
            if wait > 0:
                time.sleep(wait)
    except KeyboardInterrupt:
        print(f"\nОстановлено после {round_no} раундов")


if __name__ == "__main__":
    try:
        main()
    except requests.exceptions.RequestException as e:
        print(f"Ошибка соединения: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Ошибка: {e}", file=sys.stderr)
        sys.exit(1)
