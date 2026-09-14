#!/usr/bin/env python3
"""
Создаёт два аккаунта, пополняет первый через faucet и отправляет
три транзакции по 1 токену на второй.
"""

import json
import random
import sys
import time

import requests

API_URL = "http://localhost:1337/app"


def rpc(method, params):
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": random.randint(1000, 9999),
    }
    r = requests.post(API_URL, json=payload, timeout=10)
    if r.status_code != 200:
        raise RuntimeError(f"{method} HTTP {r.status_code}: {r.text}")
    body = r.json()
    if "error" in body:
        raise RuntimeError(f"{method}: {body['error']}")
    return body.get("result")


def create_account(name):
    result = rpc("cerera.account.create", [f"{name}_pass"])
    if not result or "address" not in result:
        raise RuntimeError(f"не удалось создать аккаунт {name}: {result}")
    return result


def get_balance(address):
    result = rpc("cerera.account.getBalance", [address])
    return float(result or 0)


def faucet(address, amount):
    return rpc("cerera.account.faucet", [address, amount])


def send_tx(sender, to_addr, amount, message=""):
    return rpc(
        "cerera.transaction.send",
        [sender["priv"], to_addr, amount, 21000, message],
    )


def main():
    print("Создание двух аккаунтов...")
    account1 = create_account("sender")
    account2 = create_account("receiver")

    addr1 = account1["address"]
    addr2 = account2["address"]
    print(f"Аккаунт 1 (отправитель): {addr1}")
    print(f"Аккаунт 2 (получатель):  {addr2}")

    print("\nFaucet 10 на аккаунт 1...")
    faucet_result = faucet(addr1, 10)
    print(f"Faucet: {faucet_result}")
    time.sleep(0.5)
    print(f"Баланс 1: {get_balance(addr1)}")
    print(f"Баланс 2: {get_balance(addr2)}")

    print("\nОтправка трёх транзакций по 1 токену...")
    for i in range(1, 4):
        tx_hash = send_tx(account1, addr2, 1, f"test_send #{i}")
        print(f"  tx {i}: {tx_hash}")

    time.sleep(0.5)
    print(f"\nБаланс 1: {get_balance(addr1)}")
    print(f"Баланс 2: {get_balance(addr2)}")


if __name__ == "__main__":
    try:
        main()
    except requests.exceptions.RequestException as e:
        print(f"Ошибка соединения с {API_URL}: {e}", file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        print(f"Ошибка: {e}", file=sys.stderr)
        sys.exit(1)
