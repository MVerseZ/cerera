# Флоу транзакции при RPC send

## Обзор
Документ описывает полный путь транзакции от момента отправки через RPC `cerera.transaction.send` до её выполнения в блоке.

## Схема флоу

```
┌─────────────────────────────────────────────────────────────────┐
│ 1. HTTP Request (RPC)                                           │
│    POST /rpc                                                     │
│    {                                                             │
│      "method": "cerera.transaction.send",                       │
│      "params": [{key, toHex, amount, gas, msg}]                │
│    }                                                             │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Network Handler                                               │
│    internal/cerera/network/handler.go                            │
│    - HandleRequest() получает HTTP запрос                        │
│    - Парсит JSON body в types.Request                           │
│    - Вызывает Execute(request.Method, request.Params)           │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Service Registry                                              │
│    internal/cerera/network/http.go                               │
│    - Execute() вызывает service.Exec()                           │
│    internal/cerera/service/registry.go                           │
│    - ParseMethod("cerera.transaction.send")                     │
│      → возвращает ("transaction", "send")                       │
│    - GetService("transaction") → Validator service              │
│    - Вызывает validator.Exec("send", params)                    │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 4. Validator.Exec("send")                                       │
│    internal/cerera/validator/validator.go:824                   │
│    - Парсит параметры (SendTxParams или legacy)                │
│    - Получает nonce: gigea.GetAndIncrementNonce()                │
│    - Создает транзакцию:                                        │
│      types.CreateUnbroadcastTransactionWei(...)                 │
│    - Подписывает транзакцию:                                    │
│      SignRawTransactionWithKey(tx, key)                         │
│      - Получает приватный ключ из vault                        │
│      - Подписывает через types.SignTx()                         │
│    - Добавляет в pool:                                          │
│      pool.Get().QueueTransaction(tx)                            │
│    - Возвращает tx.Hash()                                       │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 5. Transaction Pool                                              │
│    internal/cerera/pool/pool.go:267                              │
│    - QueueTransaction() → AddRawTransaction()                   │
│    - Проверки:                                                  │
│      * pool не переполнен (len < maxSize)                       │
│      * gas >= minGas                                            │
│    - Добавляет в memPool map[Hash]Transaction                   │
│    - Добавляет в Prepared[]                                     │
│    - Уведомляет observers: NotifyAll(tx)                        │
│    - Обновляет метрики (poolSize, poolBytes)                    │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 6. Miner (Mining Cycle)                                          │
│    internal/cerera/miner/miner.go:240                           │
│    - mineBlock() вызывается периодически                        │
│    - Получает pending транзакции:                               │
│      pool.GetPendingTransactions()                              │
│    - Создает новый блок:                                        │
│      createNewBlock(lastBlock, pendingTxs)                      │
│      - Создает header с height+1                               │
│      - Добавляет все pending транзакции                         │
│      - Добавляет coinbase транзакцию                            │
│    - Майнит блок (ищет nonce):                                  │
│      performMining(newBlock)                                    │
│    - Предлагает блок:                                           │
│      validator.ProposeBlock(newBlock)                           │
│    - Очищает pool:                                              │
│      pool.RemoveFromPool(tx.Hash()) для каждой tx               │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 7. Validator.ProposeBlock()                                      │
│    internal/cerera/validator/validator.go:424                    │
│    - Проверяет блок на валидность                               │
│    - Проверяет консенсус (Ice, consensus)                       │
│    - Для каждой транзакции в блоке:                            │
│      ExecuteTransaction(btx)                                    │
│      UpdateTxTree(&btx, blockIndex)                            │
│    - Обновляет цепочку:                                         │
│      UpdateChain(b)                                             │
└──────────────────────┬──────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│ 8. ExecuteTransaction()                                          │
│    internal/cerera/validator/validator.go:189                    │
│    - Определяет тип транзакции:                                │
│      * FaucetTxType → DropFaucet()                              │
│      * CoinbaseTxType → RewardMiner()                           │
│      * LegacyTxType → обычная транзакция                       │
│    - Для LegacyTxType:                                          │
│      - Получает отправителя: types.Sender()                    │
│      - Проверяет баланс отправителя                            │
│      - Вычитает value + gas из баланса отправителя             │
│      - Добавляет value в баланс получателя                     │
│      - Выполняет контракт (если есть)                          │
│    - Добавляет транзакцию в TxTable:                            │
│      storage.GetTxTable().Add(&tx)                              │
│    - Обновляет метрики (valExecuteSuccess)                      │
└─────────────────────────────────────────────────────────────────┘
```

## Детальное описание этапов

### 1. HTTP Request
- **Файл**: `internal/cerera/network/handler.go:52`
- Клиент отправляет JSON-RPC запрос на endpoint `/rpc`
- Метод: `cerera.transaction.send`
- Параметры: `{key, toHex, amount, gas, msg}`

### 2. Network Handler
- **Файл**: `internal/cerera/network/handler.go:52`
- Обрабатывает HTTP запрос
- Парсит JSON body
- Вызывает `Execute(method, params)`

### 3. Service Registry Routing
- **Файл**: `internal/cerera/service/registry.go:26`
- `ParseMethod("cerera.transaction.send")` → `("transaction", "send")`
- `GetService("transaction")` → возвращает Validator service
- Вызывает `validator.Exec("send", params)`

### 4. Transaction Creation & Signing
- **Файл**: `internal/cerera/validator/validator.go:824`
- **Создание транзакции**:
  - Получает nonce через `gigea.GetAndIncrementNonce()`
  - Создает транзакцию: `types.CreateUnbroadcastTransactionWei()`
- **Подписание** (`SignRawTransactionWithKey`):
  - Получает приватный ключ из vault по `key` (B58 публичный ключ)
  - Парсит PEM формат
  - Подписывает через `types.SignTx()`
- **Добавление в pool**: `pool.Get().QueueTransaction(tx)`
- **Возврат**: `tx.Hash()`

### 5. Transaction Pool
- **Файл**: `internal/cerera/pool/pool.go:267`
- **QueueTransaction()** → **AddRawTransaction()**
- **Валидация**:
  - Проверка размера pool: `len(memPool) < maxSize`
  - Проверка минимального gas: `tx.Gas() >= minGas`
- **Добавление**:
  - `memPool[tx.Hash()] = tx`
  - `Prepared = append(Prepared, tx)`
  - Уведомление observers через `NotifyAll(tx)`
- **Метрики**: обновление `poolSize`, `poolBytes`

### 6. Miner Mining Cycle
- **Файл**: `internal/cerera/miner/miner.go:240`
- **mineBlock()** вызывается периодически
- **Получение транзакций**:
  - `pool.GetPendingTransactions()` - возвращает все транзакции из memPool
- **Создание блока**:
  - `createNewBlock(lastBlock, pendingTxs)`
  - Создает header с `height = lastHeight + 1`
  - Добавляет все pending транзакции
  - Добавляет coinbase транзакцию для майнера
- **Майнинг**:
  - `performMining(newBlock)` - устанавливает nonce и вычисляет hash
- **Предложение блока**:
  - `validator.ProposeBlock(newBlock)`
- **Очистка pool**:
  - Удаляет обработанные транзакции из pool

### 7. Block Proposal & Validation
- **Файл**: `internal/cerera/validator/validator.go:424`
- **ProposeBlock()**:
  - Проверяет валидность блока
  - Проверяет консенсус (Ice, consensus status)
  - Для каждой транзакции:
    - `ExecuteTransaction(btx)` - выполняет транзакцию
    - `UpdateTxTree(&btx, blockIndex)` - добавляет в TxTable
  - `UpdateChain(b)` - добавляет блок в цепочку

### 8. Transaction Execution
- **Файл**: `internal/cerera/validator/validator.go:189`
- **ExecuteTransaction()**:
  - Определяет тип транзакции
  - **LegacyTxType** (обычная транзакция):
    - Получает адрес отправителя: `types.Sender(signer, tx)`
    - Проверяет баланс отправителя
    - Вычитает `value + gas` из баланса отправителя
    - Добавляет `value` в баланс получателя
    - Выполняет контракт (если `tx.To()` - контракт)
  - **FaucetTxType**: добавляет средства получателю
  - **CoinbaseTxType**: награждает майнера
  - Добавляет транзакцию в `TxTable` для индексации

## Ключевые компоненты

### Transaction Pool
- **Тип**: `map[common.Hash]types.GTransaction`
- **Максимальный размер**: настраивается через config
- **Минимальный gas**: настраивается через config
- **Observers**: уведомляются о новых транзакциях

### Validator
- Обрабатывает RPC методы: `create`, `send`, `get`
- Выполняет транзакции
- Валидирует блоки
- Управляет состоянием (vault, TxTable)

### Miner
- Периодически создает новые блоки
- Берет транзакции из pool
- Майнит блок (ищет nonce)
- Предлагает блок через validator

## Важные замечания

1. **Nonce**: автоматически инкрементируется через `gigea.GetAndIncrementNonce()`
2. **Подписание**: происходит на этапе `send`, не на этапе создания
3. **Валидация баланса**: происходит при выполнении транзакции, не при добавлении в pool
4. **Gas**: проверяется минимальный gas при добавлении в pool
5. **Удаление из pool**: происходит после успешного майнинга блока
6. **TxTable**: хранит индекс транзакций по блокам для быстрого поиска
