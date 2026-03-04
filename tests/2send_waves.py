import base64
import time
import requests
import json
import random
from typing import Dict

class CereraWavesTester:
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
        print("🔧 Настройка двух аккаунтов для стресс-теста...")
        
        # Создаем два аккаунта
        account1 = self.create_account("waves_tester_1", "123")
        account2 = self.create_account("waves_tester_2", "123")
        
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
    
    def calculate_delay(self, transaction_count: int, start_delay: float, end_delay: float, 
                       transactions_per_step: int = 10) -> float:
        """Вычисляет текущую задержку на основе номера транзакции"""
        # Определяем, на каком шаге мы находимся (0-based)
        step = transaction_count // transactions_per_step
        
        # Вычисляем количество шагов для перехода от start_delay к end_delay
        # Используем шаг изменения 0.1 сек между уровнями
        delay_range = abs(end_delay - start_delay)
        step_size = 0.1  # Изменение задержки на каждом шаге
        max_steps = max(1, int(delay_range / step_size))
        
        # Если мы уже прошли все шаги, возвращаем конечную задержку
        if step >= max_steps:
            return end_delay
        
        # Линейная интерполяция между start_delay и end_delay
        progress = step / max_steps if max_steps > 0 else 0
        
        if start_delay < end_delay:
            # Увеличиваем задержку
            current_delay = start_delay + (end_delay - start_delay) * progress
        else:
            # Уменьшаем задержку
            current_delay = start_delay - (start_delay - end_delay) * progress
        
        return round(current_delay, 2)
    
    def run_waves_transfer(self, amount: float = 0.1, start_delay: float = 0.1, 
                          end_delay: float = 1.5, transactions_per_step: int = 10, 
                          initial_blocks: int = 0) -> None:
        """Пересылает средства между двумя аккаунтами с изменяющимся интервалом"""
        if not self.accounts:
            print("❌ Аккаунты не настроены. Сначала выполните setup_two_accounts()")
            return
            
        account1 = self.accounts['account1']
        account2 = self.accounts['account2']
        address1 = account1['address']
        address2 = account2['address']
        
        print(f"🌊 Запуск волновой пересылки между двумя аккаунтами")
        print(f"   Сумма: {amount}")
        print(f"   Начальная задержка: {start_delay} сек")
        print(f"   Конечная задержка: {end_delay} сек")
        print(f"   Шаг изменения: каждые {transactions_per_step} транзакций")
        print(f"   Нажмите Ctrl+C для остановки")
        print("-" * 50)

        transaction_count = 0
        direction = True  # True: 1→2, False: 2→1
        current_delay = start_delay

        try:
            while True:
                # Вычисляем текущую задержку
                current_delay = self.calculate_delay(
                    transaction_count, 
                    start_delay, 
                    end_delay, 
                    transactions_per_step
                )
                
                message = "WAVES TEST " + str(transaction_count)
                
                if direction:
                    # Отправляем от аккаунта 1 к аккаунту 2
                    success = self.send_transaction(
                        account1,
                        address2,
                        amount,
                        message=message
                    )
                else:
                    # Отправляем от аккаунта 2 к аккаунту 1
                    success = self.send_transaction(
                        account2,
                        address1,
                        amount,
                        message=message
                    )

                if success:
                    transaction_count += 1
                    direction = not direction  # Меняем направление

                    # Показываем статистику каждые transactions_per_step транзакций
                    if transaction_count % transactions_per_step == 0:
                        print(f"\n📊 Выполнено транзакций: {transaction_count}")
                        print(f"⏱️  Текущая задержка: {current_delay:.2f} сек")
                        self.show_balances()
                        # Показываем текущее количество блоков
                        current_blocks = self.get_block_count()
                        print(f"   Текущих блоков в цепочке: {current_blocks}")
                        print("-" * 30)
                    else:
                        # Показываем текущую задержку для каждой транзакции
                        print(f"⏱️  Задержка: {current_delay:.2f} сек")
                
                time.sleep(current_delay)
                
        except KeyboardInterrupt:
            print(f"\n\n⏹️ Остановлено пользователем")
            print(f"📊 Итоговые результаты:")
            print(f"   Всего выполнено транзакций: {transaction_count}")
            print(f"   Финальная задержка: {current_delay:.2f} сек")
            self.show_balances()
            # Показываем финальную статистику блокчейна
            final_blocks = self.get_block_count()
            print(f"   Финальное количество блоков: {final_blocks}")
            print(f"   Блоков добавлено за тест: {final_blocks - initial_blocks}")
    
    def run_interactive_test(self) -> None:
        """Интерактивный режим тестирования"""
        print("🌊 Cerera Waves Transfer Stress Tester")
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
        print("\n⚙️ Настройка параметров волновой пересылки:")
        print("   (Нажмите Enter для использования значений по умолчанию)")
        
        try:
            amount = float(input("Введите сумму для пересылки (по умолчанию 0.1): ") or "0.1")
            
            # Запрашиваем интервал времени
            use_custom = input("Использовать кастомный интервал 0.1-1.5 сек? (y/n, по умолчанию y): ").strip().lower()
            
            if use_custom in ['', 'y', 'yes', 'да']:
                # Используем кастомный интервал
                start_delay = 0.1
                end_delay = 1.5
                transactions_per_step = 10
                print(f"✅ Используется кастомный интервал: {start_delay} - {end_delay} сек")
                print(f"   Шаг изменения: каждые {transactions_per_step} транзакций")
            else:
                # Запрашиваем начальный и конечный интервал
                start_delay = float(input("Введите начальную задержку в секундах (по умолчанию 0.1): ") or "0.1")
                end_delay = float(input("Введите конечную задержку в секундах (по умолчанию 1.5): ") or "1.5")
                transactions_per_step_input = input(f"Введите количество транзакций на шаг (по умолчанию 10): ") or "10"
                transactions_per_step = int(transactions_per_step_input)
                
                if start_delay > end_delay:
                    print("⚠️  Начальная задержка больше конечной, меняем местами")
                    start_delay, end_delay = end_delay, start_delay
                    
        except ValueError as e:
            print(f"❌ Неверный ввод: {e}, используем значения по умолчанию")
            amount = 0.1
            start_delay = 0.1
            end_delay = 1.5
            transactions_per_step = 10
            
        print(f"\n⚙️ Параметры тестирования:")
        print(f"   Сумма: {amount}")
        print(f"   Начальная задержка: {start_delay} сек")
        print(f"   Конечная задержка: {end_delay} сек")
        print(f"   Шаг изменения: каждые {transactions_per_step} транзакций")
        
        input("\nНажмите Enter для начала волновой пересылки...")
        
        # Запускаем волновую пересылку
        self.run_waves_transfer(amount, start_delay, end_delay, transactions_per_step, block_count)

def main():
    """Основная функция"""
    tester = CereraWavesTester()
    
    try:
        tester.run_interactive_test()
    except KeyboardInterrupt:
        print("\n\n⏹️ Тестирование прервано пользователем")
    except Exception as e:
        print(f"\n❌ Неожиданная ошибка: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()

