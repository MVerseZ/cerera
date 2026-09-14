# Genesis / PoW Hash Tool

Утилита для проверки **двух разных хешей** genesis-блока и поиска валидного PoW-nonce.

## Два хеша

| Хеш | Алгоритм | Где используется |
|-----|----------|------------------|
| **Chain genesis hash** | `CrvBlockHash` (tx + `Header.Bytes()`) | `genesisHash` в p2p status/sync |
| **PoW hash** | `CalculateHash` (`blake2b(ToBytes())`) | майнер, `VerifyBlockHash` |

Инструмент показывает оба. Не путайте их при отладке sync.

## Запуск

Из корня репозитория:

```bash
go run ./core/block/hash_check_tool
```

Показать текущий genesis и проверить PoW:

```bash
go run ./core/block/hash_check_tool -chainid=11
```

Найти валидный nonce (использует difficulty из `genesis.go`):

```bash
go run ./core/block/hash_check_tool -find
```

С другой сложностью (override):

```bash
go run ./core/block/hash_check_tool -find -difficulty=500
```

## Флаги

| Флаг | По умолчанию | Описание |
|------|--------------|----------|
| `-chainid` | 11 | Chain ID |
| `-difficulty` | 0 | Override difficulty (0 = из `genesis.go`) |
| `-nonce` | 0 | Override nonce (0 = из `genesis.go`) |
| `-find` | false | Искать валидный PoW nonce |
| `-max` | 10000000 | Лимит перебора nonce |

## Заметки

- Genesis в ноде: PoW **не проверяется** на height 0 (`SkipPoWAtGenesis`), но difficulty наследуется следующими блоками.
- Текущий devnet genesis: `Difficulty=1000`, валидный PoW-nonce подобран через `-find`.
