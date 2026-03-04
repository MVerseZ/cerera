import base64
import time
import requests
import json
import random
from typing import Dict

class CereraCycleTester:
    def __init__(self, api_url: str = "http://localhost:1337/app"):
        self.api_url = api_url
        self.accounts: Dict[str, Dict] = {}
        
    def create_account(self, account_id: str, password: str) -> Dict:
        """Создает новый аккаунт"""
        data_req = {
            "method": "cerera.account.create",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [password]
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                acc = json.loads(r.text)
                res = acc.get('result')
                if not res:
                    print(f"❌ Пустой результат при создании аккаунта {account_id}: {acc}")
                    return None
                print(f"✅ Создан аккаунт {account_id}: {res['address']}")
                print(f"✅ Ключ восстановления {account_id}: {res['mnemonic']}")
                return res
            else:
                print(f"❌ Ошибка создания аккаунта {account_id}: {r.text}")
                return None
        except Exception as e:
            print(f"❌ Исключение при создании аккаунта {account_id}: {e}")
            return None
    
    def send_transaction(self, sender, to_addr: str, amount: float, 
                        gas_limit: float = 1000, message: str = "") -> bool:
        """Отправляет транзакцию"""
        data_req = {
            "method": "cerera.transaction.send",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [sender['priv'], to_addr, amount, gas_limit, message]
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                print(f"✅ Отправлено {amount} от {sender['address'][:8]}... к {to_addr[:8]}...")
                return True
            else:
                print(f"❌ Ошибка отправки: {r.text}")
                return False
        except Exception as e:
            print(f"❌ Исключение при отправке: {e}")
            return False
    
    def get_chain_info(self) -> Dict:
        """Получает информацию о блокчейне"""
        data_req = {
            "method": "cerera.chain.getInfo",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": []
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                result = json.loads(r.text)
                return result.get('result', {})
            else:
                print(f"❌ Ошибка получения информации о блокчейне: {r.text}")
                return {}
        except Exception as e:
            print(f"❌ Исключение при получении информации о блокчейне: {e}")
            return {}
    
    def get_block_count(self) -> int:
        """Получает высоту цепочки (height)"""
        data_req = {
            "method": "cerera.chain.height",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": []
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                result = json.loads(r.text)
                return int(result.get('result', 0))
            else:
                print(f"❌ Ошибка получения количества блоков: {r.text}")
                return 0
        except Exception as e:
            print(f"❌ Исключение при получении количества блоков: {e}")
            return 0
    
    def get_version(self) -> str:
        """Получает версию узла"""
        data_req = {
            "method": "cerera.validator.getVersion",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": []
        }
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                result = json.loads(r.text)
                val = result.get('result')
                if val:
                    return val
        except Exception:
            pass
        return 'Unknown'
    
    def get_balance(self, address: str) -> float:
        """Получает баланс аккаунта"""
        data_req = {
            "method": "cerera.account.getBalance",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [address]
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                result = json.loads(r.text)
                return float(result.get('result', 0))
            else:
                print(f"❌ Ошибка получения баланса: {r.text}")
                return 0.0
        except Exception as e:
            print(f"❌ Исключение при получении баланса: {e}")
            return 0.0

    def faucet(self, address: str, amount: float) -> bool:
        """Выдаёт средства из крана (faucet) на указанный адрес"""
        data_req = {
            "method": "cerera.account.faucet",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [address, amount]
        }
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                print(f"🚰 Faucet: выдано {amount} на {address[:12]}...")
                return True
            else:
                print(f"❌ Ошибка faucet: {r.text}")
                return False
        except Exception as e:
            print(f"❌ Исключение faucet: {e}")
            return False
    
    def setup_two_accounts(self) -> bool:
        """Создает или использует два аккаунта для тестирования"""
        print("🔧 Настройка двух аккаунтов для циклического теста...")
        
        # Создаем два аккаунта
        account1 = self.create_account("cycle_tester_1", "123")
        account2 = self.create_account("cycle_tester_2", "123")
        
        if not account1 or not account2:
            print("❌ Не удалось создать аккаунты")
            return False
            
        self.accounts = {
            'account1': account1,
            'account2': account2
        }
        
        print(f"✅ Аккаунт 1: {account1['address'][:12]}...")
        print(f"✅ Аккаунт 2: {account2['address'][:12]}...")

        # Пополняем аккаунты через faucet перед пересылкой
        self.faucet(account1['address'], 100.0)
        self.faucet(account2['address'], 100.0)
        
        return True
    
    def show_balances(self) -> None:
        """Показывает текущие балансы обоих аккаунтов"""
        if not self.accounts:
            print("❌ Аккаунты не настроены")
            return
            
        balance1 = self.get_balance(self.accounts['account1']['address'])
        balance2 = self.get_balance(self.accounts['account2']['address'])
        total = balance1 + balance2
        
        print(f"\n💰 Балансы:")
        print(f"   Аккаунт 1: {balance1:.6f}")
        print(f"   Аккаунт 2: {balance2:.6f}")
        print(f"   Общий: {total:.6f}")
    
    def run_cycle_transfer(self, tx_count: int, amount: float = 0.1, delay: float = 0.1, initial_blocks: int = 0) -> None:
        """Циклически пересылает средства между двумя аккаунтами"""
        if not self.accounts:
            print("❌ Аккаунты не настроены. Сначала выполните setup_two_accounts()")
            return
            
        account1 = self.accounts['account1']
        account2 = self.accounts['account2']
        address1 = account1['address']
        address2 = account2['address']
        
        print(f"🔄 Запуск циклической пересылки между двумя аккаунтами")
        print(f"   Количество транзакций за цикл: {tx_count}")
        print(f"   Сумма: {amount}")
        print(f"   Задержка: {delay} сек")
        print(f"   Нажмите Ctrl+C для остановки")
        print("-" * 50)

        total_transaction_count = 0
        cycle_number = 0

        try:
            while True:
                cycle_number += 1
                print(f"\n🔄 Цикл #{cycle_number}")
                print(f"   → Отправка {tx_count} транзакций от аккаунта 1 к аккаунту 2...")
                
                # Отправляем tx_count транзакций от аккаунта 1 к аккаунту 2
                for i in range(tx_count):
                    message = f"CYCLE_{cycle_number}_1to2_TX_{i+1}"
                    success = self.send_transaction(
                        account1,
                        address2,
                        amount,
                        message=message
                    )
                    if success:
                        total_transaction_count += 1
                    time.sleep(delay)
                
                print(f"   ✅ Завершено {tx_count} транзакций 1→2")
                
                # Показываем промежуточную статистику
                if total_transaction_count % 10 == 0:
                    self.show_balances()
                    current_blocks = self.get_block_count()
                    print(f"   Текущих блоков в цепочке: {current_blocks}")
                
                print(f"\n   ← Отправка {tx_count} транзакций от аккаунта 2 к аккаунту 1...")
                
                # Отправляем tx_count транзакций от аккаунта 2 к аккаунту 1
                for i in range(tx_count):
                    message = f"CYCLE_{cycle_number}_2to1_TX_{i+1}"
                    success = self.send_transaction(
                        account2,
                        address1,
                        amount,
                        message=message
                    )
                    if success:
                        total_transaction_count += 1
                    time.sleep(delay)
                
                print(f"   ✅ Завершено {tx_count} транзакций 2→1")
                
                # Показываем статистику после каждого цикла
                print(f"\n📊 Статистика после цикла #{cycle_number}:")
                print(f"   Всего выполнено транзакций: {total_transaction_count}")
                self.show_balances()
                current_blocks = self.get_block_count()
                print(f"   Текущих блоков в цепочке: {current_blocks}")
                print("-" * 50)
                
        except KeyboardInterrupt:
            print(f"\n\n⏹️ Остановлено пользователем")
            print(f"📊 Итоговые результаты:")
            print(f"   Завершено циклов: {cycle_number}")
            print(f"   Всего выполнено транзакций: {total_transaction_count}")
            self.show_balances()
            # Показываем финальную статистику блокчейна
            final_blocks = self.get_block_count()
            print(f"   Финальное количество блоков: {final_blocks}")
            print(f"   Блоков добавлено за тест: {final_blocks - initial_blocks}")
    
    def run_interactive_test(self) -> None:
        """Интерактивный режим тестирования"""
        print("🚀 Cerera Cycle Transfer Tester")
        print("=" * 50)
        
        # Получаем информацию о системе
        print("📊 Информация о системе:")
        version = self.get_version()
        print(f"   Версия узла: {version}")
        
        chain_info = self.get_chain_info()
        if chain_info:
            print(f"   Информация о блокчейне: {chain_info}")
        
        block_count = self.get_block_count()
        print(f"   Количество блоков: {block_count}")
        print("-" * 30)
        
        # Настройка аккаунтов
        if not self.setup_two_accounts():
            return
        
        # Показываем начальные балансы
        print("\n📊 Начальные балансы:")
        self.show_balances()
        
        # Настройка параметров
        try:
            tx_count = int(input("\nВведите количество транзакций за цикл (по умолчанию 5): ") or "5")
            amount = float(input("Введите сумму для пересылки (по умолчанию 0.1): ") or "0.1")
            delay = float(input("Введите задержку между транзакциями в секундах (по умолчанию 0.1): ") or "0.1")
        except ValueError:
            print("❌ Неверный ввод, используем значения по умолчанию")
            tx_count = 5
            amount = 0.1
            delay = 0.1
            
        print(f"\n⚙️ Параметры тестирования:")
        print(f"   Количество транзакций за цикл: {tx_count}")
        print(f"   Сумма: {amount}")
        print(f"   Задержка: {delay} сек")
        
        input("\nНажмите Enter для начала циклической пересылки...")
        
        # Запускаем циклическую пересылку
        self.run_cycle_transfer(tx_count, amount, delay, block_count)

def main():
    """Основная функция"""
    tester = CereraCycleTester()
    
    try:
        tester.run_interactive_test()
    except KeyboardInterrupt:
        print("\n\n⏹️ Тестирование прервано пользователем")
    except Exception as e:
        print(f"\n❌ Неожиданная ошибка: {e}")

if __name__ == "__main__":
    main()

