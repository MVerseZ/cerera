#!/usr/bin/env python3
"""
Скрипт для создания аккаунтов в Cerera blockchain через RPC API.

Использование:
    python create_accounts.py --count 10
    python create_accounts.py -c 5 --url http://localhost:1337/app
    python create_accounts.py -c 10 --prefix "user" --output accounts.json
"""

import argparse
import json
import random
import sys
import time
from typing import List, Dict, Optional

import requests


class AccountCreator:
    """Класс для создания аккаунтов через Cerera RPC API"""
    
    def __init__(self, api_url: str = "http://localhost:1337/app"):
        """
        Инициализация AccountCreator
        
        Args:
            api_url: URL RPC endpoint
        """
        self.api_url = api_url
        self.created_accounts: List[Dict] = []
    
    def create_account(self, passphrase: str, account_id: Optional[str] = None) -> Optional[Dict]:
        """
        Создает новый аккаунт
        
        Args:
            passphrase: Парольная фраза для аккаунта
            account_id: Опциональный идентификатор аккаунта (для логирования)
        
        Returns:
            Словарь с данными аккаунта или None при ошибке
        """
        account_id = account_id or passphrase
        
        data_req = {
            "method": "cerera.account.create",
            "jsonrpc": "2.0",
            "id": random.randint(1000, 9999),
            "params": [passphrase]
        }
        
        try:
            r = requests.post(self.api_url, json=data_req, timeout=10)
            if r.status_code == 200:
                response = json.loads(r.text)
                if 'result' in response:
                    account = response['result']
                    account['account_id'] = account_id
                    account['passphrase'] = passphrase
                    self.created_accounts.append(account)
                    return account
                else:
                    print(f"❌ Ошибка создания аккаунта {account_id}: {response}")
                    return None
            else:
                print(f"❌ HTTP ошибка при создании аккаунта {account_id}: {r.status_code} - {r.text}")
                return None
        except requests.exceptions.RequestException as e:
            print(f"❌ Ошибка соединения при создании аккаунта {account_id}: {e}")
            return None
        except json.JSONDecodeError as e:
            print(f"❌ Ошибка парсинга JSON для аккаунта {account_id}: {e}")
            return None
        except Exception as e:
            print(f"❌ Неожиданная ошибка при создании аккаунта {account_id}: {e}")
            return None
    
    def create_multiple_accounts(self, count: int, prefix: str = "account", 
                                delay: float = 0.1) -> List[Dict]:
        """
        Создает несколько аккаунтов
        
        Args:
            count: Количество аккаунтов для создания
            prefix: Префикс для идентификаторов аккаунтов
            delay: Задержка между запросами в секундах
        
        Returns:
            Список созданных аккаунтов
        """
        print(f"🔧 Создание {count} аккаунтов...")
        print(f"   API URL: {self.api_url}")
        print(f"   Префикс: {prefix}")
        print("-" * 60)
        
        successful = 0
        failed = 0
        
        for i in range(count):
            account_id = f"{prefix}_{i}"
            passphrase = f"{prefix}_pass_{i}"
            
            account = self.create_account(passphrase, account_id)
            
            if account:
                successful += 1
                print(f"✅ [{i+1}/{count}] Создан аккаунт {account_id}")
                print(f"   Адрес: {account['address']}")
                if 'mnemonic' in account:
                    print(f"   Mnemonic: {account['mnemonic']}")
            else:
                failed += 1
                print(f"❌ [{i+1}/{count}] Не удалось создать аккаунт {account_id}")
            
            # Задержка между запросами
            if i < count - 1:
                time.sleep(delay)
        
        print("-" * 60)
        print(f"📊 Результаты:")
        print(f"   ✅ Успешно создано: {successful}")
        print(f"   ❌ Ошибок: {failed}")
        print(f"   📝 Всего: {len(self.created_accounts)}")
        
        return self.created_accounts
    
    def save_to_file(self, filename: str) -> bool:
        """
        Сохраняет созданные аккаунты в JSON файл
        
        Args:
            filename: Имя файла для сохранения
        
        Returns:
            True если успешно, False при ошибке
        """
        try:
            with open(filename, 'w', encoding='utf-8') as f:
                json.dump({
                    'accounts': self.created_accounts,
                    'total': len(self.created_accounts),
                    'created_at': time.strftime('%Y-%m-%d %H:%M:%S')
                }, f, indent=2, ensure_ascii=False)
            print(f"💾 Аккаунты сохранены в файл: {filename}")
            return True
        except Exception as e:
            print(f"❌ Ошибка при сохранении в файл {filename}: {e}")
            return False
    
    def print_summary(self):
        """Выводит краткую сводку по созданным аккаунтам"""
        if not self.created_accounts:
            print("⚠️  Нет созданных аккаунтов для вывода сводки")
            return
        
        print("\n📋 Сводка по созданным аккаунтам:")
        print("=" * 80)
        print(f"{'№':<4} {'ID':<20} {'Address':<45} {'Has Mnemonic':<12}")
        print("-" * 80)
        
        for i, acc in enumerate(self.created_accounts, 1):
            account_id = acc.get('account_id', 'N/A')
            address = acc.get('address', 'N/A')
            has_mnemonic = '✅' if 'mnemonic' in acc and acc['mnemonic'] else '❌'
            print(f"{i:<4} {account_id:<20} {address:<45} {has_mnemonic:<12}")
        
        print("=" * 80)
        print(f"Всего аккаунтов: {len(self.created_accounts)}")


def main():
    """Основная функция с парсингом аргументов командной строки"""
    parser = argparse.ArgumentParser(
        description='Создание аккаунтов в Cerera blockchain через RPC API',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Примеры использования:
  %(prog)s --count 10
  %(prog)s -c 5 --url http://localhost:1337/app
  %(prog)s -c 10 --prefix "user" --output accounts.json
  %(prog)s -c 20 --delay 0.2 --verbose
        """
    )
    
    parser.add_argument(
        '-c', '--count',
        type=int,
        required=True,
        help='Количество аккаунтов для создания'
    )
    
    parser.add_argument(
        '--url',
        type=str,
        default='http://localhost:1337/app',
        help='URL RPC endpoint (по умолчанию: http://localhost:1337/app)'
    )
    
    parser.add_argument(
        '--prefix',
        type=str,
        default='account',
        help='Префикс для идентификаторов аккаунтов (по умолчанию: account)'
    )
    
    parser.add_argument(
        '-o', '--output',
        type=str,
        default=None,
        help='Имя файла для сохранения результатов в JSON формате'
    )
    
    parser.add_argument(
        '--delay',
        type=float,
        default=0.1,
        help='Задержка между запросами в секундах (по умолчанию: 0.1)'
    )
    
    parser.add_argument(
        '-v', '--verbose',
        action='store_true',
        help='Подробный вывод (показывать mnemonic для каждого аккаунта)'
    )
    
    args = parser.parse_args()
    
    # Валидация аргументов
    if args.count <= 0:
        print("❌ Количество аккаунтов должно быть больше 0")
        sys.exit(1)
    
    if args.delay < 0:
        print("❌ Задержка не может быть отрицательной")
        sys.exit(1)
    
    # Создание экземпляра AccountCreator
    creator = AccountCreator(api_url=args.url)
    
    # Создание аккаунтов
    try:
        accounts = creator.create_multiple_accounts(
            count=args.count,
            prefix=args.prefix,
            delay=args.delay
        )
        
        # Вывод подробной информации если включен verbose режим
        if args.verbose:
            print("\n📝 Подробная информация об аккаунтах:")
            print("=" * 80)
            for i, acc in enumerate(accounts, 1):
                print(f"\nАккаунт #{i}:")
                print(f"  ID: {acc.get('account_id', 'N/A')}")
                print(f"  Address: {acc.get('address', 'N/A')}")
                if 'mnemonic' in acc:
                    print(f"  Mnemonic: {acc['mnemonic']}")
                if 'pub' in acc:
                    print(f"  Public Key: {acc['pub'][:50]}...")
        
        # Вывод сводки
        creator.print_summary()
        
        # Сохранение в файл если указано
        if args.output:
            creator.save_to_file(args.output)
        
        print("\n🎉 Готово!")
        
    except KeyboardInterrupt:
        print("\n\n⏹️  Прервано пользователем")
        if creator.created_accounts:
            print(f"⚠️  Создано {len(creator.created_accounts)} аккаунтов до прерывания")
            if args.output:
                creator.save_to_file(args.output)
        sys.exit(0)
    except Exception as e:
        print(f"\n❌ Неожиданная ошибка: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
