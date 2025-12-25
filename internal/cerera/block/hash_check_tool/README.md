# Block Hash Check Tool

Утилита для вычисления и проверки хэша блока на соответствие требованиям difficulty.

## Компиляция

Из корня проекта:
```bash
go build -o hash_check.exe ./internal/cerera/block/hash_check_tool
```

Или из директории `hash_check_tool`:
```bash
go build -o hash_check.exe
```

## Использование

### Базовое использование (с параметрами по умолчанию):
```bash
./hash_check.exe
```

### С кастомными параметрами:
```bash
./hash_check.exe -chainid=11 -difficulty=1000 -nonce=12345
```

### Параметры:
- `-chainid` - Chain ID для genesis блока (по умолчанию: 11)
- `-difficulty` - Значение difficulty (0 = использовать значение из genesis)
- `-nonce` - Значение nonce (0 = использовать значение из genesis)

## Пример вывода

```
=== Block Hash Calculator and Verifier ===

Using genesis difficulty: 11111111111111111
Using genesis nonce: 5437
Block Height: 0
Block Chain ID: 11

Block Hash (hex): a1b2c3d4e5f6...
Block Hash (bytes length: 32)

✓ Block hash is VALID (meets difficulty requirement)

=== Detailed Verification ===
Block Hash Verification:
  Status: VALID
  Height: 0
  Difficulty: 11111111111111111
  Nonce: 5437
  ...
```

