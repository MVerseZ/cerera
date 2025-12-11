import base64
import time
import requests
import json
import random
from typing import List, Dict

class CereraTester:
    def __init__(self, api_url: str = "http://localhost:1337/app"):
        self.api_url = api_url
        self.accounts: List[Dict] = []
        
    def create_account(self, account_id: str, password: str) -> Dict:
        """Создает новый аккаунт"""
        data_req = {
            "method": "cerera.account.create",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [account_id, password]
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                acc = json.loads(r.text)
                print(f"✅ Создан аккаунт {account_id}: {acc['result']['address']}")
                return acc['result']
            else:
                print(f"❌ Ошибка создания аккаунта {account_id}: {r.text}")
                return None
        except Exception as e:
            print(f"❌ Исключение при создании аккаунта {account_id}: {e}")
            return None
    
    def send_transaction(self, sender, to_addr: str, amount: float, 
                        gas_limit: int = 50000, message: str = "") -> bool:
        """Отправляет транзакцию"""
        data_req = {
            "method": "cerera.transaction.send",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [sender['pub'], to_addr, amount, gas_limit, message]
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

    def get_mempool_info(self) -> Dict:
        """Получает информацию о мемпуле"""
        data_req = {
            "method": "cerera.pool.getInfo",
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
                print(f"❌ Ошибка получения мемпула: {r.text}")
                return {}
        except Exception as e:
            print(f"❌ Исключение при получении мемпула: {e}")
            return {}
    
    def create_multiple_accounts(self, count: int) -> List[Dict]:
        """Создает несколько аккаунтов"""
        print(f"🔧 Создание {count} аккаунтов...")
        accounts = []
        
        for i in range(count):
            account = self.create_account(f"user_{i}", f"pass_{i}")
            if account:
                accounts.append(account)
            time.sleep(0.1)  # Небольшая задержка между запросами
            
        print(f"✅ Создано {len(accounts)} аккаунтов из {count} запрошенных")
        return accounts
    
    def perform_random_transactions(self, accounts: List[Dict], 
                                  transaction_count: int = 10) -> None:
        """Выполняет случайные транзакции между аккаунтами"""
        print(f"🔄 Выполнение {transaction_count} случайных транзакций...")
        
        successful_tx = 0
        for i in range(transaction_count):
            if len(accounts) < 2:
                print("❌ Недостаточно аккаунтов для транзакций")
                break
                
            # Выбираем случайного отправителя и получателя
            sender = random.choice(accounts)
            receiver = random.choice([acc for acc in accounts if acc != sender])
            
            # Случайная сумма от 0.1 до 2.0
            amount = round(random.uniform(0.1, 2.0), 2)
            
            # Отправляем транзакцию
            if self.send_transaction(
                sender, 
                receiver['address'], 
                amount,
                message=f"Transaction #{i+1}"
            ):
                successful_tx += 1
                
            time.sleep(0.02)  # Задержка между транзакциями
            
        print(f"✅ Успешно выполнено {successful_tx} из {transaction_count} транзакций")
    
    def show_accounts_summary(self, accounts: List[Dict], title: str = "Сводка по аккаунтам") -> Dict[str, float]:
        """Показывает сводку по аккаунтам и возвращает словарь с балансами"""
        print(f"\n📊 {title}:")
        print("-" * 60)
        
        balances = {}
        total_balance = 0.0
        
        for i, account in enumerate(accounts):
            balance = self.get_balance(account['address'])
            balances[account['address']] = balance
            total_balance += balance
            print(f"Аккаунт {i+1}: {account['address'][:12]}... | Баланс: {balance}")
            time.sleep(0.1)
        
        print("-" * 60)
        print(f"💰 Общий баланс всех аккаунтов: {total_balance}")
        
        return balances
    
    def show_balance_changes(self, accounts: List[Dict], initial_balances: Dict[str, float], 
                           final_balances: Dict[str, float]) -> None:
        """Показывает изменения балансов между начальным и финальным состоянием"""
        print("\n📈 Изменения балансов:")
        print("-" * 80)
        
        # Вычисляем общие балансы
        initial_total = sum(initial_balances.values())
        final_total = sum(final_balances.values())
        total_change = final_total - initial_total
        
        changes_count = 0
        
        for i, account in enumerate(accounts):
            address = account['address']
            initial = initial_balances.get(address, 0.0)
            final = final_balances.get(address, 0.0)
            change = final - initial
            
            if change != 0:
                changes_count += 1
                change_symbol = "📈" if change > 0 else "📉"
                print(f"Аккаунт {i+1}: {address[:12]}... | {initial:.2f} → {final:.2f} | {change_symbol} {change:+.2f}")
            else:
                print(f"Аккаунт {i+1}: {address[:12]}... | {initial:.2f} → {final:.2f} | ➖ без изменений")
        
        print("-" * 80)
        print(f"📊 Статистика изменений:")
        print(f"   • Аккаунтов с изменениями: {changes_count} из {len(accounts)}")
        print(f"   • Общий баланс: {initial_total:.2f} → {final_total:.2f} | {total_change:+.2f}")
        
        if total_change == 0:
            print("   ✅ Общий баланс системы сохранен!")
        else:
            print("   ⚠️ Обнаружено изменение общего баланса системы")
    
    def run_interactive_test(self) -> None:
        """Интерактивный режим тестирования"""
        print("🚀 Cerera Blockchain Tester")
        print("=" * 40)
        
        # Создание аккаунтов
        try:
            account_count = int(input("Введите количество аккаунтов для создания: "))
        except ValueError:
            print("❌ Неверный ввод, используем значение по умолчанию: 5")
            account_count = 5
            
        accounts = self.create_multiple_accounts(account_count)
        
        if not accounts:
            print("❌ Не удалось создать аккаунты. Завершение работы.")
            return
            
        # Показываем начальную сводку
        initial_balances = self.show_accounts_summary(accounts, "Начальные балансы")
        
        # Транзакции
        try:
            tx_count = int(input("\nВведите количество транзакций для выполнения: "))
        except ValueError:
            print("❌ Неверный ввод, используем значение по умолчанию: 10")
            tx_count = 10
            
        input("\nНажмите Enter для начала выполнения транзакций...")
        self.perform_random_transactions(accounts, tx_count)
        
        # Финальная сводка
        final_balances = self.show_accounts_summary(accounts, "Финальные балансы")
        
        # Показываем изменения балансов
        self.show_balance_changes(accounts, initial_balances, final_balances)
        
        print("\n🎉 Тестирование завершено!")

    def run_two_accounts_flow(self) -> None:
        """Создает 2 аккаунта, отправляет транзакции 1→2 и проверяет мемпул"""
        print("🚀 Cerera Two-Accounts Flow")
        print("=" * 40)
        # 1. Создает аккаунт 1
        acc1 = self.create_account("user_1", "pass_1")
        if not acc1:
            print("❌ Не удалось создать аккаунт 1")
            return
        # 2. Создает аккаунт 2
        acc2 = self.create_account("user_2", "pass_2")
        if not acc2:
            print("❌ Не удалось создать аккаунт 2")
            return
        self.accounts = [acc1, acc2]

        # Показать начальные балансы
        b1 = self.get_balance(acc1['address'])
        b2 = self.get_balance(acc2['address'])
        print(f"Начальные балансы -> A1: {b1}, A2: {b2}")

        # 3. Посылает транзакции с одного на другой
        sent = 0
        for i in range(100):
            ok = self.send_transaction(acc1, acc2['address'], amount=0.1, gas_limit=21000, message=f"tx #{i+1}")
            if ok:
                sent += 1
            time.sleep(0.1)
        print(f"✅ Отправлено транзакций: {sent}")

        # 4. Проверяет мемпул
        # mp = self.get_mempool_info()
        # size = mp.get('Size') or mp.get('size') or 0
        # hashes = mp.get('Hashes') or mp.get('hashes') or []
        # print(f"🧰 Мемпул -> size: {size}, hashes: {hashes}")

        # Финальные балансы
        fb1 = self.get_balance(acc1['address'])
        fb2 = self.get_balance(acc2['address'])
        print(f"Финальные балансы -> A1: {fb1}, A2: {fb2}")

def main():
    """Основная функция"""
    tester = CereraTester()
    
    try:
        tester.run_two_accounts_flow()
    except KeyboardInterrupt:
        print("\n\n⏹️ Тестирование прервано пользователем")
    except Exception as e:
        print(f"\n❌ Неожиданная ошибка: {e}")

if __name__ == "__main__":
    main()
