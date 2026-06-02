import base64
import time
import requests
import json
import random
from typing import Dict, List, Tuple

# Используем существующий класс CereraWavesTester
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
                return res
            else:
                print(f"❌ Ошибка создания аккаунта {account_id}: {r.text}")
                return None
        except Exception as e:
            print(f"❌ Исключение при создании аккаунта {account_id}: {e}")
            return None
    
    def send_transaction(self, sender, to_addr: str, amount: float, 
                        gas_limit: float = 1000, message: str = "") -> Tuple[bool, str]:
        """Отправляет транзакцию. Возвращает (успех, tx_hash/ошибка)"""
        data_req = {
            "method": "cerera.transaction.send",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [sender['priv'], to_addr, amount, gas_limit, message]
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                result = json.loads(r.text)
                tx_hash = result.get('result', {}).get('hash', 'unknown')
                print(f"✅ Отправлено {amount} от {sender['address'][:8]}... к {to_addr[:8]}... хеш: {tx_hash}")
                return True, tx_hash
            else:
                print(f"❌ Ошибка отправки: {r.text}")
                return False, r.text
        except Exception as e:
            print(f"❌ Исключение при отправке: {e}")
            return False, str(e)
    
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
    
    def wait_for_blocks(self, target_blocks: int = 1, timeout: int = 60) -> bool:
        """Ожидает появления target_blocks новых блоков"""
        start_height = self.get_block_count()
        print(f"⏳ Ожидание появления {target_blocks} новых блоков (текущая высота {start_height})...")
        start_time = time.time()
        while time.time() - start_time < timeout:
            current_height = self.get_block_count()
            if current_height >= start_height + target_blocks:
                print(f"✅ Достигнута высота {current_height}")
                return True
            time.sleep(2)
        print(f"❌ Таймаут: не появилось {target_blocks} новых блоков за {timeout} сек")
        return False

# Новый класс для тестирования множества аккаунтов
class CereraMultiAccountTester(CereraWavesTester):
    def __init__(self, api_url: str = "http://localhost:1337/app"):
        super().__init__(api_url)
        self.account_list: List[Dict] = []  # список аккаунтов (каждый как dict с address, priv, mnemonic)
    
    def create_multiple_accounts(self, n: int) -> bool:
        """Создает n аккаунтов и сохраняет их в self.account_list"""
        print(f"\n🔧 Создание {n} аккаунтов...")
        for i in range(1, n+1):
            acc = self.create_account(f"user_{i}", f"pass_{i}")
            if not acc:
                print(f"❌ Не удалось создать аккаунт {i}")
                return False
            self.account_list.append(acc)
            # Небольшая задержка между созданиями, чтобы не перегружать API
            time.sleep(0.5)
        print(f"✅ Успешно создано {len(self.account_list)} аккаунтов")
        return True
    
    def fund_all_accounts(self, amount: float = 100.0) -> bool:
        """Выдаёт faucet каждому аккаунту"""
        print(f"\n💰 Запрос faucet для всех аккаунтов ({amount} единиц каждому)...")
        for idx, acc in enumerate(self.account_list):
            if not self.faucet(acc['address'], amount):
                print(f"❌ Не удалось пополнить аккаунт {idx+1}")
                return False
            time.sleep(0.5)  # пауза между запросами
        print("✅ Все аккаунты пополнены")
        return True
    
    def show_all_balances(self) -> List[float]:
        """Выводит балансы всех аккаунтов и возвращает список балансов"""
        balances = []
        print("\n💰 Текущие балансы аккаунтов:")
        for idx, acc in enumerate(self.account_list):
            bal = self.get_balance(acc['address'])
            balances.append(bal)
            print(f"   Аккаунт {idx+1}: {bal:.6f} ({acc['address'][:12]}...)")
        return balances
    
    def perform_varied_transfers(self, min_amount: float = 0.5, max_amount: float = 5.0) -> List[Tuple[int, int, float, str]]:
        """
        Выполняет разнообразные переводы между аккаунтами.
        Каждый аккаунт переводит каждому другому случайную сумму в заданном диапазоне.
        Возвращает список выполненных переводов: (отправитель_index, получатель_index, сумма, хеш)
        """
        n = len(self.account_list)
        if n < 2:
            print("❌ Нужно хотя бы 2 аккаунта для переводов")
            return []
        
        print(f"\n🔄 Выполнение разнообразных переводов между {n} аккаунтами...")
        print(f"   Суммы: случайные от {min_amount} до {max_amount}")
        
        transfers = []
        total_tx = n * (n-1)  # полносвязная схема
        current = 0
        
        for i in range(n):
            for j in range(n):
                if i == j:
                    continue
                amount = round(random.uniform(min_amount, max_amount), 6)
                message = f"Transfer from {i+1} to {j+1} amount {amount}"
                success, tx_hash = self.send_transaction(
                    self.account_list[i],
                    self.account_list[j]['address'],
                    amount,
                    message=message
                )
                if success:
                    transfers.append((i, j, amount, tx_hash))
                else:
                    print(f"⚠️ Перевод от {i+1} к {j+1} не удался")
                current += 1
                # Прогресс
                print(f"   Прогресс: {current}/{total_tx} переводов")
                time.sleep(0.2)  # небольшая пауза, чтобы не залить сеть
        
        print(f"✅ Выполнено успешных переводов: {len(transfers)} из {total_tx}")
        return transfers
    
    def verify_delivery_and_balances(self, transfers: List[Tuple[int, int, float, str]], 
                                     initial_balances: List[float]) -> bool:
        """
        Проверяет, что все переводы доставлены и балансы корректны.
        Ожидает подтверждения в блоках, затем сравнивает ожидаемые балансы с реальными.
        """
        if not transfers:
            print("❌ Нет переводов для проверки")
            return False
        
        print("\n🔍 Ожидание подтверждения транзакций в блокчейне...")
        # Ждём появления нескольких блоков (например, 3), чтобы все транзакции точно попали
        if not self.wait_for_blocks(target_blocks=3, timeout=90):
            print("⚠️ Не удалось дождаться новых блоков, но проверим балансы всё равно")
        
        # Вычисляем ожидаемые балансы
        n = len(self.account_list)
        expected_balances = initial_balances.copy()
        for (from_idx, to_idx, amount, _) in transfers:
            expected_balances[from_idx] -= amount
            expected_balances[to_idx] += amount
            # Небольшая комиссия? В Cerera может быть gas, но в данном тесте мы её игнорируем,
            # т.к. считается, что faucet выдал достаточно, и gas списывается отдельно.
            # Для точности можно уменьшить баланс отправителя ещё на gas_limit (но gas_limit не учтён в сумме).
            # Упрощённо считаем, что комиссия незначительна или уже включена.
        
        # Получаем реальные балансы
        real_balances = self.show_all_balances()
        
        # Сравниваем
        all_ok = True
        tolerance = 1e-5
        print("\n📊 Проверка корректности балансов:")
        for i in range(n):
            expected = expected_balances[i]
            real = real_balances[i]
            diff = abs(expected - real)
            if diff > tolerance:
                print(f"   ❌ Аккаунт {i+1}: ожидалось {expected:.6f}, получено {real:.6f} (разница {diff:.6f})")
                all_ok = False
            else:
                print(f"   ✅ Аккаунт {i+1}: {real:.6f} (ожидалось {expected:.6f})")
        
        if all_ok:
            print("\n✅ Все переводы доставлены, балансы соответствуют ожидаемым!")
        else:
            print("\n⚠️ Некоторые балансы не совпадают. Возможно, транзакции ещё не подтверждены или есть комиссии.")
        return all_ok
    
    def run_multi_account_test(self):
        """Основной сценарий: создание n аккаунтов, запрос faucet, переводы, проверка"""
        print("🌊 Cerera Multi-Account Tester")
        print("=" * 50)
        
        # Получаем информацию о системе
        version = self.get_version()
        print(f"Версия узла: {version}")
        block_count = self.get_block_count()
        print(f"Количество блоков: {block_count}")
        print("-" * 30)
        
        # Ввод количества аккаунтов
        while True:
            try:
                n = int(input("Введите количество аккаунтов для создания (минимум 2): ") or "3")
                if n >= 2:
                    break
                else:
                    print("❌ Введите число не менее 2")
            except ValueError:
                print("❌ Введите целое число")
        
        # Шаг 1: создание n аккаунтов
        if not self.create_multiple_accounts(n):
            print("❌ Не удалось создать необходимое количество аккаунтов")
            return
        
        # Шаг 2: запрос faucet для всех
        if not self.fund_all_accounts(amount=100.0):
            print("❌ Не удалось пополнить все аккаунты")
            return
        
        # Ждём, чтобы faucet обработался (появление блоков)
        print("\n⏳ Ожидание обработки faucet-транзакций...")
        self.wait_for_blocks(target_blocks=2, timeout=60)
        
        # Фиксируем начальные балансы
        initial_balances = self.show_all_balances()
        
        # Шаг 3: отправка между аккаунтами с разными суммами
        transfers = self.perform_varied_transfers(min_amount=0.5, max_amount=5.0)
        
        if not transfers:
            print("❌ Не выполнено ни одной успешной транзакции")
            return
        
        # Шаг 4: проверка доставки и нового баланса
        self.verify_delivery_and_balances(transfers, initial_balances)
        
        print("\n🏁 Тест завершён.")

def main():
    tester = CereraMultiAccountTester()
    try:
        tester.run_multi_account_test()
    except KeyboardInterrupt:
        print("\n\n⏹️ Тестирование прервано пользователем")
    except Exception as e:
        print(f"\n❌ Неожиданная ошибка: {e}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    main()