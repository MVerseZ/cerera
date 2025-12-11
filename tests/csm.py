#!/usr/bin/env python3
"""
CSM (Create, Send, Monitor) Script для тестирования Cerera blockchain
Создает аккаунт, отправляет транзакцию и проверяет ее статус
"""

import requests
import json
import time
import random
from typing import Dict, Optional


class CereraCSM:
    def __init__(self, api_url: str = "http://localhost:1337/app"):
        self.api_url = api_url
        self.session = requests.Session()
        self.session.headers.update({'Content-Type': 'application/json'})
    
    def create_account(self, account_id: str = "", password: str = "") -> Optional[Dict]:
        """Создает новый аккаунт"""
        print("🔧 Создание аккаунта...")
        
        data_req = {
            "method": "cerera.account.create",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [account_id, password]
        }
        
        try:
            response = self.session.post(self.api_url, json=data_req, timeout=10)
            if response.status_code == 200:
                result = response.json()
                if 'result' in result:
                    account = result['result']
                    print(f"✅ Аккаунт создан успешно!")
                    print(f"   Адрес: {account['address']}")
                    print(f"   Публичный ключ: {account['pub'][:50]}...")
                    print(f"   Мнемоника: {account['mnemonic'][:50]}...")
                    return account
                else:
                    print(f"❌ Ошибка создания аккаунта: {result}")
                    return None
            else:
                print(f"❌ HTTP ошибка: {response.status_code} - {response.text}")
                return None
        except Exception as e:
            print(f"❌ Исключение при создании аккаунта: {e}")
            return None
    
    def get_balance(self, address: str) -> float:
        """Получает баланс аккаунта"""
        data_req = {
            "method": "get_balance",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [address]
        }
        
        try:
            response = self.session.post(self.api_url, json=data_req, timeout=10)
            if response.status_code == 200:
                result = response.json()
                balance = float(result.get('result', 0))
                print(f"💰 Баланс адреса {address[:8]}...: {balance}")
                return balance
            else:
                print(f"❌ Ошибка получения баланса: {response.text}")
                return 0.0
        except Exception as e:
            print(f"❌ Исключение при получении баланса: {e}")
            return 0.0
    
    def faucet(self, address: str, amount: float = 1000.0) -> bool:
        """Получает токены через faucet"""
        print(f"🚰 Получение {amount} токенов через faucet...")
        
        data_req = {
            "method": "faucet",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [address, amount]
        }
        
        try:
            response = self.session.post(self.api_url, json=data_req, timeout=10)
            if response.status_code == 200:
                result = response.json()
                if 'result' in result:
                    print(f"✅ Получено {amount} токенов через faucet")
                    return True
                else:
                    print(f"❌ Ошибка faucet: {result}")
                    return False
            else:
                print(f"❌ HTTP ошибка faucet: {response.status_code} - {response.text}")
                return False
        except Exception as e:
            print(f"❌ Исключение при faucet: {e}")
            return False
    
    def send_transaction(self, sender: Dict, to_address: str, amount: float, 
                        gas_limit: int = 50000, message: str = "Test transaction from CSM") -> Optional[str]:
        """Отправляет транзакцию"""
        print(f"📤 Отправка транзакции...")
        print(f"   От: {sender['address'][:8]}...")
        print(f"   К: {to_address[:8]}...")
        print(f"   Сумма: {amount}")
        print(f"   Сообщение: {message}")
        
        data_req = {
            "method": "send_tx",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [
                sender['pub'],
                to_address,
                amount,
                gas_limit,
                message
            ]
        }
        
        try:
            response = self.session.post(self.api_url, json=data_req, timeout=10)
            if response.status_code == 200:
                result = response.json()
                if 'result' in result:
                    tx_hash = result['result']
                    print(f"✅ Транзакция отправлена!")
                    print(f"   Хеш транзакции: {tx_hash}")
                    return tx_hash
                else:
                    print(f"❌ Ошибка отправки транзакции: {result}")
                    return None
            else:
                print(f"❌ HTTP ошибка отправки: {response.status_code} - {response.text}")
                return None
        except Exception as e:
            print(f"❌ Исключение при отправке транзакции: {e}")
            return None
    
    def get_transaction_status(self, tx_hash: str) -> Optional[Dict]:
        """Проверяет статус транзакции"""
        print(f"🔍 Проверка статуса транзакции {tx_hash[:8]}...")
        
        data_req = {
            "method": "get_tx",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [tx_hash]
        }
        
        try:
            response = self.session.post(self.api_url, json=data_req, timeout=10)
            if response.status_code == 200:
                result = response.json()
                if 'result' in result:
                    tx_data = result['result']
                    print(f"✅ Статус транзакции получен:")
                    print(f"   Хеш: {tx_data.get('hash', 'N/A')}")
                    print(f"   От: {tx_data.get('from', 'N/A')}")
                    print(f"   К: {tx_data.get('to', 'N/A')}")
                    print(f"   Сумма: {tx_data.get('value', 'N/A')}")
                    print(f"   Газ: {tx_data.get('gas', 'N/A')}")
                    print(f"   Nonce: {tx_data.get('nonce', 'N/A')}")
                    return tx_data
                else:
                    print(f"❌ Транзакция не найдена: {result}")
                    return None
            else:
                print(f"❌ HTTP ошибка проверки статуса: {response.status_code} - {response.text}")
                return None
        except Exception as e:
            print(f"❌ Исключение при проверке статуса: {e}")
            return None
    
    def get_mempool_info(self) -> Optional[Dict]:
        """Получает информацию о мемпуле"""
        data_req = {
            "method": "getmempoolinfo",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": []
        }
        
        try:
            response = self.session.post(self.api_url, json=data_req, timeout=10)
            if response.status_code == 200:
                result = response.json()
                if 'result' in result:
                    mempool_info = result['result']
                    print(f"📊 Информация о мемпуле:")
                    print(f"   Количество транзакций: {mempool_info}")
                    return mempool_info
                else:
                    print(f"❌ Ошибка получения информации о мемпуле: {result}")
                    return None
            else:
                print(f"❌ HTTP ошибка мемпула: {response.status_code} - {response.text}")
                return None
        except Exception as e:
            print(f"❌ Исключение при получении мемпула: {e}")
            return None


def main():
    """Основная функция выполнения CSM теста"""
    print("🚀 Запуск CSM (Create, Send, Monitor) теста для Cerera")
    print("=" * 60)
    
    # Инициализация клиента
    csm = CereraCSM()
    
    # Шаг 1: Создание аккаунта
    print("\n📝 ШАГ 1: СОЗДАНИЕ АККАУНТА")
    print("-" * 30)
    account = csm.create_account("csm_test_account", "test_password")
    if not account:
        print("❌ Не удалось создать аккаунт. Завершение теста.")
        return
    
    # Шаг 2: Получение токенов через faucet
    print("\n💰 ШАГ 2: ПОЛУЧЕНИЕ ТОКЕНОВ ЧЕРЕЗ FAUCET")
    print("-" * 40)
    faucet_success = csm.faucet(account['address'], 1000.0)
    if not faucet_success:
        print("⚠️  Faucet не сработал, но продолжаем тест...")
    
    # Проверяем баланс после faucet
    time.sleep(2)  # Небольшая пауза для обработки
    balance = csm.get_balance(account['address'])
    
    # Шаг 3: Создание второго аккаунта для отправки
    print("\n📝 ШАГ 3: СОЗДАНИЕ ВТОРОГО АККАУНТА")
    print("-" * 35)
    receiver_account = csm.create_account("csm_receiver_account", "test_password")
    if not receiver_account:
        print("❌ Не удалось создать второй аккаунт. Завершение теста.")
        return
    
    # Шаг 4: Отправка транзакции
    print("\n📤 ШАГ 4: ОТПРАВКА ТРАНЗАКЦИИ")
    print("-" * 30)
    if balance > 0:
        amount_to_send = min(10.0, balance * 0.1)  # Отправляем 10 токенов или 10% от баланса
        tx_hash = csm.send_transaction(
            sender=account,
            to_address=receiver_account['address'],
            amount=amount_to_send,
            message="CSM Test Transaction"
        )
        
        if tx_hash:
            # Шаг 5: Проверка статуса транзакции
            print("\n🔍 ШАГ 5: ПРОВЕРКА СТАТУСА ТРАНЗАКЦИИ")
            print("-" * 40)
            
            # Ждем немного перед проверкой
            print("⏳ Ожидание обработки транзакции...")
            time.sleep(3)
            
            # Проверяем статус несколько раз
            for attempt in range(3):
                print(f"\n🔍 Попытка {attempt + 1}/3:")
                tx_status = csm.get_transaction_status(tx_hash)
                if tx_status:
                    print("✅ Транзакция найдена в блокчейне!")
                    break
                else:
                    if attempt < 2:
                        print("⏳ Транзакция еще не найдена, ждем...")
                        time.sleep(5)
                    else:
                        print("⚠️  Транзакция не найдена после 3 попыток")
            
            # Проверяем балансы после транзакции
            print("\n💰 ПРОВЕРКА БАЛАНСОВ ПОСЛЕ ТРАНЗАКЦИИ")
            print("-" * 40)
            time.sleep(2)
            sender_balance = csm.get_balance(account['address'])
            receiver_balance = csm.get_balance(receiver_account['address'])
            
            print(f"\n📊 ИТОГОВЫЕ РЕЗУЛЬТАТЫ:")
            print(f"   Отправитель ({account['address'][:8]}...): {sender_balance}")
            print(f"   Получатель ({receiver_account['address'][:8]}...): {receiver_balance}")
            
        else:
            print("❌ Не удалось отправить транзакцию")
    else:
        print("❌ Недостаточно средств для отправки транзакции")
    
    # Дополнительная информация
    print("\n📊 ДОПОЛНИТЕЛЬНАЯ ИНФОРМАЦИЯ")
    print("-" * 30)
    csm.get_mempool_info()
    
    print("\n✅ CSM тест завершен!")
    print("=" * 60)


if __name__ == "__main__":
    main()
