# PeerInfo - Расширенная информация о пирах

## Обзор

Структура `PeerInfo` была добавлена для поддержки как адресов Cerera (`types.Address`), так и сетевых адресов (IP:port) в системе консенсуса.

## Структура

```go
type PeerInfo struct {
    CereraAddress types.Address  // Адрес Cerera (криптографический адрес)
    NetworkAddr   string         // Сетевой адрес в формате "IP:port"
}
```

## Использование

### Создание PeerInfo

```go
// Создание из адреса Cerera и сетевого адреса
cereraAddr := types.HexToAddress("0x1234567890123456789012345678901234567890")
networkAddr := "192.168.1.100:8080"
peerInfo := NewPeerInfo(cereraAddr, networkAddr)
```

### Добавление пира в ConsensusManager

```go
// Старый способ (только адрес Cerera)
consensusManager.AddPeer(cereraAddress)

// Новый способ (с сетевым адресом)
peerInfo := NewPeerInfo(cereraAddress, "192.168.1.100:8080")
consensusManager.AddPeer(peerInfo)
```

### Инициализация ConsensusManager

```go
// Создание списка пиров с сетевыми адресами
peers := []*PeerInfo{
    NewPeerInfo(nodeID, "localhost:8080"),
    NewPeerInfo(peer1Addr, "192.168.1.100:8081"),
    NewPeerInfo(peer2Addr, "192.168.1.101:8082"),
}

consensusManager := NewConsensusManager(ConsensusTypeSimple, nodeID, peers, engine)
```

## Важные особенности

### Сетевой адрес текущей ноды

При автоматическом создании PeerInfo (например, в Engine.AddPeer) используется сетевой адрес текущей ноды:

```go
// В Engine.AddPeer и Engine.UpdatePeer
peerInfo := &PeerInfo{
    CereraAddress: peer,
    NetworkAddr:   fmt.Sprintf(":%d", e.Port), // Используется сетевой адрес текущей ноды
}
```

Это означает, что все пиры по умолчанию подключаются к сетевому адресу текущей ноды, что является правильным поведением для P2P сети.

## Преимущества

1. **Полная информация о пирах**: Теперь система хранит как криптографический адрес, так и сетевой адрес
2. **Гибкость**: Можно указывать произвольные сетевые адреса вместо автоматического вычисления
3. **Обратная совместимость**: Старые методы AddPeer с types.Address все еще работают
4. **Лучшая отладка**: Легче отслеживать подключения по конкретным сетевым адресам
5. **Правильное сетевое поведение**: Используется сетевой адрес текущей ноды для подключений

## Миграция

Для существующего кода:

1. Замените вызовы `AddPeer(types.Address)` на `AddPeer(*PeerInfo)`
2. Обновите инициализацию ConsensusManager для использования `[]*PeerInfo`
3. Используйте `NewPeerInfo()` для создания объектов PeerInfo

## Тестирование

Все тесты были обновлены для работы с новой структурой PeerInfo:

### Обновленные тесты

- `network_test.go` - тесты сетевого взаимодействия
- `consensus_test.go` - тесты алгоритмов консенсуса
- `peerinfo_test.go` - новые тесты для функциональности PeerInfo

### Пример теста

```go
func TestPeerInfoWithConsensusManager(t *testing.T) {
    // Create test addresses
    node1 := types.HexToAddress("0x1111111111111111111111111111111111111111")
    node2 := types.HexToAddress("0x2222222222222222222222222222222222222222")

    // Create PeerInfo list
    peers := []*PeerInfo{
        NewPeerInfo(node1, "localhost:30001"),
        NewPeerInfo(node2, "localhost:30002"),
    }

    // Create consensus manager
    consensusManager := NewConsensusManager(ConsensusTypeSimple, node1, peers, engine)
    
    // Test that peers were added correctly
    if len(consensusManager.Peers) != 2 {
        t.Errorf("Expected 2 peers, got %d", len(consensusManager.Peers))
    }
}
```

## Примеры

Смотрите файл `example_usage.go` для полных примеров использования.
