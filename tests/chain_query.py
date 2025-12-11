#!/usr/bin/env python3
"""
Утилита для запросов к файлу chain.dat
Поддерживает чтение блоков, поиск, статистику и экспорт данных
"""

import json
import argparse
import sys
from pathlib import Path
from datetime import datetime
from typing import List, Dict, Optional, Any
from collections import defaultdict


class ChainQuery:
    """Класс для работы с chain.dat файлом"""
    
    def __init__(self, chain_file: str = "chain.dat"):
        """
        Инициализация утилиты
        
        Args:
            chain_file: Путь к файлу chain.dat
        """
        self.chain_file = Path(chain_file)
        if not self.chain_file.exists():
            # Попробуем найти в корне проекта
            root_chain = Path(__file__).parent.parent / "chain.dat"
            if root_chain.exists():
                self.chain_file = root_chain
            else:
                raise FileNotFoundError(f"Файл chain.dat не найден: {chain_file}")
    
    def read_blocks(self) -> List[Dict[str, Any]]:
        """
        Читает все блоки из файла
        
        Returns:
            Список блоков в виде словарей
        """
        blocks = []
        try:
            with open(self.chain_file, 'r', encoding='utf-8') as f:
                for line_num, line in enumerate(f, 1):
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        block = json.loads(line)
                        blocks.append(block)
                    except json.JSONDecodeError as e:
                        print(f"Ошибка парсинга строки {line_num}: {e}", file=sys.stderr)
                        continue
        except Exception as e:
            print(f"Ошибка чтения файла: {e}", file=sys.stderr)
            sys.exit(1)
        
        return blocks
    
    def get_block_by_height(self, height: int) -> Optional[Dict[str, Any]]:
        """
        Получает блок по высоте
        
        Args:
            height: Высота блока
            
        Returns:
            Блок или None
        """
        blocks = self.read_blocks()
        for block in blocks:
            if block.get('header', {}).get('height') == height:
                return block
        return None
    
    def get_block_by_hash(self, block_hash: str) -> Optional[Dict[str, Any]]:
        """
        Получает блок по хешу
        
        Args:
            block_hash: Хеш блока (с префиксом 0x или без)
            
        Returns:
            Блок или None
        """
        if not block_hash.startswith('0x'):
            block_hash = '0x' + block_hash
        
        blocks = self.read_blocks()
        for block in blocks:
            if block.get('hash', '').lower() == block_hash.lower():
                return block
        return None
    
    def get_latest_block(self) -> Optional[Dict[str, Any]]:
        """
        Получает последний блок
        
        Returns:
            Последний блок или None
        """
        blocks = self.read_blocks()
        if not blocks:
            return None
        
        # Находим блок с максимальной высотой
        latest = max(blocks, key=lambda b: b.get('header', {}).get('height', -1))
        return latest
    
    def get_blocks_range(self, start: int, end: Optional[int] = None) -> List[Dict[str, Any]]:
        """
        Получает блоки в диапазоне высот
        
        Args:
            start: Начальная высота
            end: Конечная высота (если None, то до конца)
            
        Returns:
            Список блоков
        """
        blocks = self.read_blocks()
        result = []
        
        for block in blocks:
            height = block.get('header', {}).get('height', -1)
            if height >= start:
                if end is None or height <= end:
                    result.append(block)
        
        return sorted(result, key=lambda b: b.get('header', {}).get('height', 0))
    
    def get_statistics(self) -> Dict[str, Any]:
        """
        Получает статистику по блокчейну
        
        Returns:
            Словарь со статистикой
        """
        blocks = self.read_blocks()
        if not blocks:
            return {
                'total_blocks': 0,
                'total_transactions': 0,
                'chain_id': None,
                'latest_height': None,
                'latest_hash': None
            }
        
        total_txs = 0
        heights = []
        chain_ids = set()
        gas_used_total = 0
        gas_limit_total = 0
        timestamps = []
        
        for block in blocks:
            header = block.get('header', {})
            heights.append(header.get('height', 0))
            chain_ids.add(header.get('chainId'))
            gas_used_total += header.get('gasUsed', 0)
            gas_limit_total += header.get('gasLimit', 0)
            timestamps.append(header.get('timestamp', 0))
            
            txs = block.get('transactions', [])
            total_txs += len(txs)
        
        latest_block = self.get_latest_block()
        
        # Вычисляем временные метрики
        if timestamps:
            first_timestamp = min(timestamps)
            last_timestamp = max(timestamps)
            time_span = (last_timestamp - first_timestamp) / 1000  # в секундах
        else:
            time_span = 0
        
        stats = {
            'total_blocks': len(blocks),
            'total_transactions': total_txs,
            'chain_id': list(chain_ids)[0] if chain_ids else None,
            'latest_height': latest_block.get('header', {}).get('height') if latest_block else None,
            'latest_hash': latest_block.get('hash') if latest_block else None,
            'height_range': {
                'min': min(heights) if heights else 0,
                'max': max(heights) if heights else 0
            },
            'gas_statistics': {
                'total_used': gas_used_total,
                'total_limit': gas_limit_total,
                'average_used_per_block': gas_used_total / len(blocks) if blocks else 0
            },
            'time_statistics': {
                'first_block_time': datetime.fromtimestamp(first_timestamp / 1000).isoformat() if timestamps else None,
                'last_block_time': datetime.fromtimestamp(last_timestamp / 1000).isoformat() if timestamps else None,
                'time_span_seconds': time_span,
                'time_span_days': time_span / 86400 if time_span > 0 else 0
            }
        }
        
        return stats
    
    def search_transactions(self, address: Optional[str] = None, 
                           tx_hash: Optional[str] = None) -> List[Dict[str, Any]]:
        """
        Ищет транзакции
        
        Args:
            address: Адрес для поиска (to или from)
            tx_hash: Хеш транзакции
            
        Returns:
            Список найденных транзакций с информацией о блоке
        """
        blocks = self.read_blocks()
        results = []
        
        for block in blocks:
            txs = block.get('transactions', [])
            for tx in txs:
                match = False
                
                if tx_hash:
                    tx_hash_clean = tx_hash.lower()
                    if not tx_hash_clean.startswith('0x'):
                        tx_hash_clean = '0x' + tx_hash_clean
                    if tx.get('hash', '').lower() == tx_hash_clean:
                        match = True
                
                if address:
                    address_clean = address.lower()
                    if not address_clean.startswith('0x'):
                        address_clean = '0x' + address_clean
                    if tx.get('to', '').lower() == address_clean:
                        match = True
                
                if match:
                    results.append({
                        'block_height': block.get('header', {}).get('height'),
                        'block_hash': block.get('hash'),
                        'transaction': tx
                    })
        
        return results
    
    def export_blocks(self, output_file: str, format: str = 'json',
                     start_height: Optional[int] = None,
                     end_height: Optional[int] = None):
        """
        Экспортирует блоки в файл
        
        Args:
            output_file: Путь к выходному файлу
            format: Формат экспорта ('json', 'jsonl')
            start_height: Начальная высота (опционально)
            end_height: Конечная высота (опционально)
        """
        if start_height is not None or end_height is not None:
            blocks = self.get_blocks_range(
                start_height or 0,
                end_height
            )
        else:
            blocks = self.read_blocks()
        
        output_path = Path(output_file)
        
        if format == 'json':
            with open(output_path, 'w', encoding='utf-8') as f:
                json.dump(blocks, f, indent=2, ensure_ascii=False)
        elif format == 'jsonl':
            with open(output_path, 'w', encoding='utf-8') as f:
                for block in blocks:
                    f.write(json.dumps(block, ensure_ascii=False) + '\n')
        else:
            raise ValueError(f"Неподдерживаемый формат: {format}")
        
        print(f"Экспортировано {len(blocks)} блоков в {output_file}")


def print_block(block: Dict[str, Any], detailed: bool = False):
    """Выводит информацию о блоке"""
    if not block:
        print("Блок не найден")
        return
    
    header = block.get('header', {})
    print(f"\n{'='*60}")
    print(f"Блок #{header.get('height', 'N/A')}")
    print(f"{'='*60}")
    print(f"Хеш: {block.get('hash', 'N/A')}")
    print(f"Предыдущий хеш: {header.get('prevHash', 'N/A')}")
    print(f"Высота: {header.get('height', 'N/A')}")
    print(f"Индекс: {header.get('index', 'N/A')}")
    print(f"Chain ID: {header.get('chainId', 'N/A')}")
    print(f"Сложность: {header.get('difficulty', 'N/A')}")
    print(f"Nonce: {header.get('nonce', 'N/A')}")
    
    timestamp = header.get('timestamp', 0)
    if timestamp:
        dt = datetime.fromtimestamp(timestamp / 1000)
        print(f"Время: {dt.isoformat()}")
    
    print(f"Gas Limit: {header.get('gasLimit', 'N/A')}")
    print(f"Gas Used: {header.get('gasUsed', 'N/A')}")
    print(f"Размер: {header.get('size', 'N/A')} байт")
    print(f"Транзакций: {len(block.get('transactions', []))}")
    print(f"Подтверждений: {block.get('confirmations', 0)}")
    
    if detailed:
        print(f"\nВерсия: {header.get('version', 'N/A')}")
        print(f"Extra Data: {header.get('extraData', 'N/A')}")
        print(f"Node: {header.get('node', 'N/A')}")
        print(f"State Root: {header.get('stateRoot', 'N/A')}")
        
        txs = block.get('transactions', [])
        if txs:
            print(f"\nТранзакции:")
            for i, tx in enumerate(txs, 1):
                print(f"  [{i}] Hash: {tx.get('hash', 'N/A')}")
                print(f"      To: {tx.get('to', 'N/A')}")
                print(f"      Value: {tx.get('value', 'N/A')}")
                print(f"      Gas: {tx.get('gas', 'N/A')}")
                print(f"      Gas Price: {tx.get('gasPrice', 'N/A')}")
    
    print(f"{'='*60}\n")


def main():
    parser = argparse.ArgumentParser(
        description='Утилита для запросов к файлу chain.dat',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Примеры использования:
  %(prog)s --stats                          # Показать статистику
  %(prog)s --block-height 100               # Получить блок по высоте
  %(prog)s --block-hash 0xabc123...         # Получить блок по хешу
  %(prog)s --latest                         # Получить последний блок
  %(prog)s --range 0 100                    # Получить блоки с 0 по 100
  %(prog)s --search-tx --address 0x123...   # Найти транзакции по адресу
  %(prog)s --export output.json --format json  # Экспортировать все блоки
        """
    )
    
    parser.add_argument(
        '--chain-file',
        type=str,
        default='chain.dat',
        help='Путь к файлу chain.dat (по умолчанию: chain.dat)'
    )
    
    # Основные команды
    parser.add_argument('--stats', action='store_true', help='Показать статистику блокчейна')
    parser.add_argument('--latest', action='store_true', help='Показать последний блок')
    parser.add_argument('--block-height', type=int, help='Получить блок по высоте')
    parser.add_argument('--block-hash', type=str, help='Получить блок по хешу')
    parser.add_argument('--range', nargs=2, type=int, metavar=('START', 'END'),
                       help='Получить блоки в диапазоне высот')
    
    # Поиск транзакций
    parser.add_argument('--search-tx', action='store_true', help='Поиск транзакций')
    parser.add_argument('--address', type=str, help='Адрес для поиска транзакций')
    parser.add_argument('--tx-hash', type=str, help='Хеш транзакции для поиска')
    
    # Экспорт
    parser.add_argument('--export', type=str, metavar='FILE', help='Экспортировать блоки в файл')
    parser.add_argument('--format', choices=['json', 'jsonl'], default='json',
                       help='Формат экспорта (по умолчанию: json)')
    parser.add_argument('--export-start', type=int, help='Начальная высота для экспорта')
    parser.add_argument('--export-end', type=int, help='Конечная высота для экспорта')
    
    # Опции вывода
    parser.add_argument('--detailed', action='store_true', help='Подробный вывод блока')
    parser.add_argument('--json', action='store_true', help='Вывести результат в формате JSON')
    
    args = parser.parse_args()
    
    try:
        query = ChainQuery(args.chain_file)
    except FileNotFoundError as e:
        print(f"Ошибка: {e}", file=sys.stderr)
        sys.exit(1)
    
    # Обработка команд
    if args.stats:
        stats = query.get_statistics()
        if args.json:
            print(json.dumps(stats, indent=2, ensure_ascii=False))
        else:
            print("\n" + "="*60)
            print("СТАТИСТИКА БЛОКЧЕЙНА")
            print("="*60)
            print(f"Всего блоков: {stats['total_blocks']}")
            print(f"Всего транзакций: {stats['total_transactions']}")
            print(f"Chain ID: {stats['chain_id']}")
            print(f"Последняя высота: {stats['latest_height']}")
            print(f"Последний хеш: {stats['latest_hash']}")
            print(f"\nДиапазон высот: {stats['height_range']['min']} - {stats['height_range']['max']}")
            print(f"\nGas статистика:")
            print(f"  Всего использовано: {stats['gas_statistics']['total_used']}")
            print(f"  Всего лимит: {stats['gas_statistics']['total_limit']}")
            print(f"  Среднее на блок: {stats['gas_statistics']['average_used_per_block']:.2f}")
            if stats['time_statistics']['first_block_time']:
                print(f"\nВременная статистика:")
                print(f"  Первый блок: {stats['time_statistics']['first_block_time']}")
                print(f"  Последний блок: {stats['time_statistics']['last_block_time']}")
                print(f"  Период: {stats['time_statistics']['time_span_days']:.2f} дней")
            print("="*60 + "\n")
    
    elif args.latest:
        block = query.get_latest_block()
        if args.json:
            print(json.dumps(block, indent=2, ensure_ascii=False))
        else:
            print_block(block, args.detailed)
    
    elif args.block_height is not None:
        block = query.get_block_by_height(args.block_height)
        if args.json:
            print(json.dumps(block, indent=2, ensure_ascii=False))
        else:
            print_block(block, args.detailed)
    
    elif args.block_hash:
        block = query.get_block_by_hash(args.block_hash)
        if args.json:
            print(json.dumps(block, indent=2, ensure_ascii=False))
        else:
            print_block(block, args.detailed)
    
    elif args.range:
        start, end = args.range
        blocks = query.get_blocks_range(start, end)
        if args.json:
            print(json.dumps(blocks, indent=2, ensure_ascii=False))
        else:
            print(f"\nНайдено блоков: {len(blocks)}")
            for block in blocks:
                print_block(block, args.detailed)
    
    elif args.search_tx:
        results = query.search_transactions(args.address, args.tx_hash)
        if args.json:
            print(json.dumps(results, indent=2, ensure_ascii=False))
        else:
            print(f"\nНайдено транзакций: {len(results)}")
            for result in results:
                print(f"\nБлок #{result['block_height']} ({result['block_hash']})")
                tx = result['transaction']
                print(f"  TX Hash: {tx.get('hash', 'N/A')}")
                print(f"  To: {tx.get('to', 'N/A')}")
                print(f"  Value: {tx.get('value', 'N/A')}")
                print(f"  Gas: {tx.get('gas', 'N/A')}")
                print(f"  Nonce: {tx.get('nonce', 'N/A')}")
    
    elif args.export:
        query.export_blocks(
            args.export,
            args.format,
            args.export_start,
            args.export_end
        )
    
    else:
        parser.print_help()


if __name__ == '__main__':
    main()

