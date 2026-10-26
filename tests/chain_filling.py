import time
import requests
import random
from typing import Any, Dict, List, Tuple

GAS_LIMIT = 1000
MIN_GAS_PRICE = 0.000001  # 1 DUST за единицу газа


def rpc_call(api_url: str, method: str, params: List) -> Any:
    """Выполняет JSON-RPC запрос и возвращает его result или понятную ошибку."""
    payload = {
        "jsonrpc": "2.0",
        "method": method,
        "id": random.randint(1000, 9999),
        "params": params,
    }
    response = requests.post(api_url, json=payload, timeout=10)
    response.raise_for_status()
    body = response.json()
    if body.get("error"):
        raise RuntimeError(f"{method}: {body['error']}")
    if "result" not in body:
        raise RuntimeError(f"{method}: в ответе отсутствует result: {body}")
    return body["result"]

# Используем существующий класс CereraWavesTester
class CereraWavesTester:
    def __init__(self, api_url: str = "http://localhost:1337/app"):
        self.api_url = api_url
        self.accounts: Dict[str, Dict] = {}
        
    def create_account(self, account_id: str, password: str) -> Dict:
        """Создает новый аккаунт"""
        try:
            result = rpc_call(self.api_url, "cerera.account.create", [password])
            if not isinstance(result, dict) or not result.get("address") or not result.get("priv"):
                raise RuntimeError(f"неполный результат создания аккаунта: {result}")
            print(f"✅ Создан аккаунт {account_id}: {result['address']}")
            return result
        except Exception as e:
            print(f"❌ Исключение при создании аккаунта {account_id}: {e}")
            return None
    
    def send_transaction(self, sender, to_addr: str, amount: float, 
                        gas_limit: float = GAS_LIMIT, message: str = "") -> Tuple[bool, str]:
        """Отправляет транзакцию. Возвращает (успех, tx_hash/ошибка)"""
        try:
            tx_hash = rpc_call(
                self.api_url,
                "cerera.transaction.send",
                [sender['priv'], to_addr, amount, gas_limit, message],
            )
            if not tx_hash:
                raise RuntimeError("пустой hash транзакции")
            print(f"✅ Отправлено {amount} от {sender['address'][:8]}... к {to_addr[:8]}... хеш: {tx_hash}")
            return True, str(tx_hash)
        except Exception as e:
            print(f"❌ Исключение при отправке: {e}")
            return False, str(e)
    
    def get_chain_info(self) -> Dict:
        """Получает информацию о блокчейне"""
        try:
            return rpc_call(self.api_url, "cerera.chain.getInfo", [])
        except Exception as e:
            print(f"❌ Исключение при получении информации о блокчейне: {e}")
            return {}
    
    def get_block_count(self) -> int:
        """Получает высоту цепочки (height)"""
        try:
            return int(rpc_call(self.api_url, "cerera.chain.height", []))
        except Exception as e:
            print(f"❌ Исключение при получении количества блоков: {e}")
            return 0
    
    def get_version(self) -> str:
        """Получает версию узла"""
        try:
            result = rpc_call(self.api_url, "cerera.validator.getVersion", [])
            if result:
                return str(result)
        except Exception:
            pass
        return 'Unknown'
    
    def get_balance(self, address: str) -> float:
        """Получает баланс аккаунта"""
        try:
            return float(rpc_call(self.api_url, "cerera.account.getBalance", [address]))
        except Exception as e:
            print(f"❌ Исключение при получении баланса: {e}")
            return 0.0

    def faucet(self, address: str, amount: float) -> bool:
        """Выдаёт средства из крана (faucet) на указанный адрес"""
        try:
            result = rpc_call(self.api_url, "cerera.account.faucet", [address, amount])
            if result:
                print(f"🚰 Faucet: выдано {amount} на {address[:12]}...")
                return True
            print(f"❌ Faucet вернул пустой результат для {address[:12]}...")
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
        gas_fee = GAS_LIMIT * MIN_GAS_PRICE
        for (from_idx, to_idx, amount, _) in transfers:
            expected_balances[from_idx] -= amount + gas_fee
            expected_balances[to_idx] += amount

        print(f"   Учтена комиссия: {gas_fee:.6f} за каждый исходящий перевод")
        
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
