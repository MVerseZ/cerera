package icenet

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/chain"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/cerera/validator"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// Метрики для синхронизации блоков
	blocksReceivedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "icenet_blocks_received_total",
		Help: "Total number of blocks received from other nodes",
	})
	blocksProcessedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "icenet_blocks_processed_total",
		Help: "Total number of blocks successfully processed and added to chain",
	})
	blocksRejectedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "icenet_blocks_rejected_total",
		Help: "Total number of blocks rejected (validation failed, duplicates, etc.)",
	})
	blocksBroadcastTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "icenet_blocks_broadcast_total",
		Help: "Total number of blocks broadcasted to other nodes",
	})
	blockSyncErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "icenet_block_sync_errors_total",
			Help: "Total number of block synchronization errors by type",
		},
		[]string{"error_type"},
	)
)

func init() {
	prometheus.MustRegister(
		blocksReceivedTotal,
		blocksProcessedTotal,
		blocksRejectedTotal,
		blocksBroadcastTotal,
		blockSyncErrorsTotal,
	)
}

// monitorAndSendBlocks периодически проверяет новые блоки и отправляет их на bootstrap
func (i *Ice) monitorAndSendBlocks() {
	ticker := time.NewTicker(DefaultBlockCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-i.ctx.Done():
			return
		case <-ticker.C:
			i.mu.RLock()
			started := i.started
			i.mu.RUnlock()

			if !started {
				return
			}

			// Получаем последний блок из chain через registry
			registry, err := service.GetRegistry()
			if err != nil {
				icelogger().Debugw("Registry not available", "err", err)
				continue
			}

			chainService, ok := registry.GetService("chain")
			if !ok {
				icelogger().Debugw("Chain service not found")
				continue
			}

			// Получаем последний блок через Exec
			result := chainService.Exec("getLatestBlock", nil)
			if result == nil {
				continue
			}

			latestBlock, ok := result.(*block.Block)
			if !ok || latestBlock == nil {
				continue
			}

			// Проверяем, не отправляли ли мы уже этот блок
			i.mu.RLock()
			alreadySent := latestBlock.Hash == i.lastSentBlockHash
			i.mu.RUnlock()
			if alreadySent {
				continue
			}

			// Рассылаем блок всем нодам
			if err := i.broadcastBlock(latestBlock); err != nil {
				// icelogger().Warnw("Failed to broadcast block",
				// 	"block_hash", latestBlock.Hash.Hex(),
				// 	"block_index", latestBlock.Head.Index,
				// 	"err", err,
				// )
				// Continue monitoring - connection will be re-established automatically
				continue
			}

			i.mu.Lock()
			i.lastSentBlockHash = latestBlock.Hash
			i.mu.Unlock()
			icelogger().Infow("Block sent to bootstrap",
				"block_hash", latestBlock.Hash.Hex(),
				"block_index", latestBlock.Head.Index,
			)
		}
	}
}

// sendBlockToBootstrap отправляет блок на bootstrap узел через постоянное соединение
// Deprecated: используйте broadcastBlock() вместо этой функции
func (i *Ice) sendBlockToBootstrap(b *block.Block) error {
	if i.isBootstrapNode() {
		return nil // Не отправляем блоки самому себе
	}

	i.mu.Lock()
	conn := i.bootstrapConn
	i.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("bootstrap connection not established")
	}

	return i.sendBlockToConnection(conn, b)
}

// broadcastBlock рассылает блок всем подключенным нодам
// Для bootstrap ноды: рассылает всем из connectedNodes
// Для обычных нод: отправляет bootstrap и другим известным нодам
func (i *Ice) broadcastBlock(b *block.Block) error {
	if b == nil {
		return fmt.Errorf("block is nil")
	}

	var errors []error
	sentCount := 0

	if i.isBootstrapNode() {
		// Bootstrap нода: рассылаем всем подключенным нодам
		i.mu.RLock()
		connectedNodes := make(map[types.Address]net.Conn)
		for addr, conn := range i.connectedNodes {
			connectedNodes[addr] = conn
		}
		i.mu.RUnlock()

		for addr, conn := range connectedNodes {
			if err := i.sendBlockToConnection(conn, b); err != nil {
				icelogger().Warnw("Failed to send block to connected node",
					"node_address", addr.Hex(),
					"block_hash", b.Hash.Hex(),
					"err", err)
				errors = append(errors, fmt.Errorf("node %s: %w", addr.Hex(), err))
			} else {
				sentCount++
			}
		}

		icelogger().Infow("Block broadcasted from bootstrap",
			"block_hash", b.Hash.Hex(),
			"block_height", b.Head.Height,
			"sent_to_nodes", sentCount,
			"total_nodes", len(connectedNodes),
		)
		// Увеличиваем счетчик для каждого отправленного блока
		for i := 0; i < sentCount; i++ {
			blocksBroadcastTotal.Inc()
		}
	} else {
		// Обычная нода: отправляем bootstrap
		i.mu.RLock()
		conn := i.bootstrapConn
		i.mu.RUnlock()

		if conn == nil {
			return fmt.Errorf("bootstrap connection not established")
		}

		if err := i.sendBlockToConnection(conn, b); err != nil {
			icelogger().Warnw("Failed to send block to bootstrap",
				"block_hash", b.Hash.Hex(),
				"err", err)
			blockSyncErrorsTotal.WithLabelValues("broadcast_error").Inc()
			return fmt.Errorf("failed to send block to bootstrap: %w", err)
		}

		sentCount++
		blocksBroadcastTotal.Inc()
		icelogger().Infow("Block sent to bootstrap",
			"block_hash", b.Hash.Hex(),
			"block_height", b.Head.Height,
		)
	}

	// Если были ошибки, но хотя бы один блок отправлен, возвращаем частичный успех
	if len(errors) > 0 && sentCount > 0 {
		icelogger().Warnw("Block broadcasted with some errors",
			"block_hash", b.Hash.Hex(),
			"sent_count", sentCount,
			"error_count", len(errors))
		return nil // Частичный успех
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to broadcast block: %d errors", len(errors))
	}

	return nil
}

// sendBlockToConnection отправляет блок через указанное соединение
func (i *Ice) sendBlockToConnection(conn net.Conn, b *block.Block) error {
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	if b == nil {
		return fmt.Errorf("block is nil")
	}

	// Сериализуем блок в JSON
	blockJSON, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	// Формируем сообщение: BLOCK|<json_data>\n
	message := fmt.Sprintf("%s|%s\n", MsgTypeBlock, string(blockJSON))

	// Отправляем блок через соединение
	conn.SetWriteDeadline(time.Now().Add(DefaultConnectionTimeout))
	if _, err := conn.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to write to connection: %w", err)
	}

	return nil
}

// processReceivedBlock обрабатывает полученный блок от другой ноды
func (i *Ice) processReceivedBlock(data string, source string) {
	// Увеличиваем счетчик полученных блоков
	blocksReceivedTotal.Inc()

	// Парсим блок из сообщения
	b, err := i.parseBlockMessage(data)
	if err != nil {
		icelogger().Warnw("Failed to parse block message",
			"source", source,
			"err", err)
		blocksRejectedTotal.Inc()
		blockSyncErrorsTotal.WithLabelValues("parse_error").Inc()
		return
	}

	icelogger().Infow("Processing received block",
		"source", source,
		"block_hash", b.Hash.Hex(),
		"block_height", b.Head.Height,
		"block_index", b.Head.Index,
	)

	// Валидируем блок
	if err := i.validateReceivedBlock(b); err != nil {
		icelogger().Warnw("Block validation failed",
			"source", source,
			"block_hash", b.Hash.Hex(),
			"block_height", b.Head.Height,
			"err", err)
		blocksRejectedTotal.Inc()
		blockSyncErrorsTotal.WithLabelValues("validation_error").Inc()
		return
	}

	// Добавляем блок в цепочку
	if err := i.addBlockToChain(b); err != nil {
		icelogger().Warnw("Failed to add block to chain",
			"source", source,
			"block_hash", b.Hash.Hex(),
			"block_height", b.Head.Height,
			"err", err)
		blocksRejectedTotal.Inc()
		blockSyncErrorsTotal.WithLabelValues("add_to_chain_error").Inc()
		return
	}

	// Блок успешно обработан
	blocksProcessedTotal.Inc()
	icelogger().Infow("Successfully processed and added block",
		"source", source,
		"block_hash", b.Hash.Hex(),
		"block_height", b.Head.Height,
		"block_index", b.Head.Index,
	)
}

// validateReceivedBlock валидирует полученный блок
func (i *Ice) validateReceivedBlock(b *block.Block) error {
	// 1. Проверка структуры блока
	if b == nil {
		return fmt.Errorf("block is nil")
	}

	if b.Head == nil {
		return fmt.Errorf("block header is nil")
	}

	header := b.Header()
	if header == nil {
		return fmt.Errorf("block header is invalid")
	}

	// 2. Проверка базовых полей
	if header.Height < 0 {
		return fmt.Errorf("invalid block height: %d", header.Height)
	}

	if header.ChainId == 0 {
		return fmt.Errorf("invalid chain ID: %d", header.ChainId)
	}

	// 3. Проверка хэша блока
	calculatedHash := block.CrvBlockHash(*b)
	if b.Hash != calculatedHash {
		return fmt.Errorf("block hash mismatch: expected %s, got %s",
			calculatedHash.Hex(), b.Hash.Hex())
	}

	// 4. Проверка PrevHash - должен соответствовать последнему блоку в цепочке
	registry, err := service.GetRegistry()
	if err != nil {
		return fmt.Errorf("failed to get registry: %w", err)
	}

	chainService, ok := registry.GetService("chain")
	if !ok {
		return fmt.Errorf("chain service not found")
	}

	latestBlockResult := chainService.Exec("getLatestBlock", nil)
	if latestBlockResult == nil {
		// Если цепочка пуста, это может быть genesis блок
		if header.Height == 0 {
			return nil
		}
		return fmt.Errorf("chain is empty but block height is %d", header.Height)
	}

	latestBlock, ok := latestBlockResult.(*block.Block)
	if !ok || latestBlock == nil {
		return fmt.Errorf("invalid latest block from chain service")
	}

	// Проверяем, не является ли это дубликатом
	i.mu.RLock()
	alreadyReceived := i.receivedBlockHashes[b.Hash]
	i.mu.RUnlock()

	if alreadyReceived {
		return fmt.Errorf("block already received: %s", b.Hash.Hex())
	}

	// Проверяем PrevHash
	if header.PrevHash != latestBlock.GetHash() {
		// Это может быть форк - обрабатываем его
		if err := i.handleChainFork(b); err != nil {
			return fmt.Errorf("fork handling failed: %w", err)
		}
		// handleChainFork решил, что блок можно добавить
	}

	// 5. Проверка транзакций (базовая)
	if b.Transactions == nil {
		// Транзакции могут быть пустыми, но не nil
		b.Transactions = make([]types.GTransaction, 0)
	}

	// 6. Проверка difficulty (базовая - должна быть >= 1)
	if header.Difficulty < 1 {
		return fmt.Errorf("invalid difficulty: %d", header.Difficulty)
	}

	// 7. Проверка nonce (proof of work)
	// В текущей реализации nonce проверяется через хэш блока
	// Если хэш корректен, значит nonce валиден

	return nil
}

// addBlockToChain безопасно добавляет блок в цепочку
func (i *Ice) addBlockToChain(b *block.Block) error {
	// Проверяем, не добавлен ли уже блок
	i.mu.Lock()
	if i.receivedBlockHashes[b.Hash] {
		i.mu.Unlock()
		return fmt.Errorf("block already in chain: %s", b.Hash.Hex())
	}
	// Помечаем блок как полученный
	i.receivedBlockHashes[b.Hash] = true
	i.mu.Unlock()

	// Используем validator.ProposeBlock для правильной обработки блока
	// ProposeBlock выполнит транзакции и добавит блок в цепочку
	val := validator.Get()
	if val == nil {
		return fmt.Errorf("validator not available")
	}

	// ProposeBlock выполнит транзакции и обновит цепочку
	val.ProposeBlock(b)

	icelogger().Infow("Block added to chain via validator",
		"block_hash", b.Hash.Hex(),
		"block_height", b.Head.Height,
		"block_index", b.Head.Index,
	)

	return nil
}

// compareChains сравнивает две цепочки и возвращает true, если удаленная цепочка лучше
// Правило: longest chain wins (самая длинная цепочка побеждает)
// При равной длине: больше chain work (суммарная сложность)
func (i *Ice) compareChains(localHeight int, localWork uint64, remoteHeight int, remoteWork uint64) bool {
	// Сначала сравниваем по высоте (longest chain rule)
	if remoteHeight > localHeight {
		return true
	}
	if remoteHeight < localHeight {
		return false
	}

	// При равной высоте сравниваем по chain work
	if remoteWork > localWork {
		return true
	}

	// Локальная цепочка лучше или равна
	return false
}

// handleChainFork обрабатывает форк цепочки
// Обнаруживает форк, сравнивает цепочки и реорганизует, если нужно
func (i *Ice) handleChainFork(newBlock *block.Block) error {
	registry, err := service.GetRegistry()
	if err != nil {
		return fmt.Errorf("failed to get registry: %w", err)
	}

	chainService, ok := registry.GetService("chain")
	if !ok {
		return fmt.Errorf("chain service not found")
	}

	// Получаем текущую цепочку
	latestBlockResult := chainService.Exec("getLatestBlock", nil)
	if latestBlockResult == nil {
		// Цепочка пуста, просто добавляем блок
		return nil
	}

	latestBlock, ok := latestBlockResult.(*block.Block)
	if !ok || latestBlock == nil {
		return fmt.Errorf("invalid latest block from chain service")
	}

	// Проверяем, не является ли новый блок следующим после последнего
	if newBlock.Head.PrevHash == latestBlock.GetHash() {
		// Это не форк, просто следующий блок
		return nil
	}

	// Обнаружен форк - PrevHash не совпадает
	icelogger().Warnw("Fork detected",
		"new_block_hash", newBlock.Hash.Hex(),
		"new_block_height", newBlock.Head.Height,
		"new_block_prev_hash", newBlock.Head.PrevHash.Hex(),
		"latest_block_hash", latestBlock.GetHash().Hex(),
		"latest_block_height", latestBlock.Header().Height,
	)

	// Получаем информацию о цепочках для сравнения
	chainInfo := chainService.Exec("getInfo", nil)
	if chainInfo == nil {
		return fmt.Errorf("failed to get chain info")
	}

	info, ok := chainInfo.(chain.BlockChainStatus)
	if !ok {
		return fmt.Errorf("invalid chain info type")
	}

	// Сравниваем цепочки
	localHeight := latestBlock.Header().Height
	localWork := uint64(info.ChainWork)
	remoteHeight := newBlock.Head.Height
	remoteWork := uint64(newBlock.Head.Size) // Упрощенная оценка work

	// Если удаленная цепочка лучше, нужно реорганизовать
	if i.compareChains(localHeight, localWork, remoteHeight, remoteWork) {
		icelogger().Infow("Remote chain is better, reorganization needed",
			"local_height", localHeight,
			"local_work", localWork,
			"remote_height", remoteHeight,
			"remote_work", remoteWork,
		)

		// В текущей реализации просто добавляем блок
		// Полная реорганизация требует более сложной логики:
		// 1. Найти общий предок
		// 2. Откатить транзакции из неканонической ветки
		// 3. Применить транзакции из канонической ветки
		// Это будет реализовано в будущем
		icelogger().Warnw("Chain reorganization not fully implemented, adding block anyway")
		return nil
	}

	// Локальная цепочка лучше, игнорируем форк
	icelogger().Infow("Local chain is better, ignoring fork",
		"local_height", localHeight,
		"local_work", localWork,
		"remote_height", remoteHeight,
		"remote_work", remoteWork,
	)

	return fmt.Errorf("local chain is better, ignoring fork")
}

// Exec выполняет методы сервиса Ice.
// Предоставляет программный интерфейс для получения информации о состоянии компонента.
// Поддерживаемые методы:
//   - "status": возвращает статус компонента (byte)
//   - "getInfo": возвращает полную информацию о компоненте (map[string]interface{})
//   - "getAddress": возвращает Cerera адрес узла (string)
//   - "getIP": возвращает IP адрес узла (string)
//   - "getPort": возвращает порт узла (string)
//   - "getNetworkAddr": возвращает сетевой адрес узла (string)
//   - "getBootstrapAddr": возвращает адрес bootstrap узла (string)
//   - "isBootstrap": возвращает true, если текущий узел является bootstrap (bool)
//   - "isBootstrapReady": возвращает готовность bootstrap (bool)
//   - "waitForBootstrapReady": блокирует до готовности bootstrap (bool)
//   - "isConsensusStarted": возвращает статус консенсуса (bool)
//   - "waitForConsensus": блокирует до начала консенсуса (bool)
//
// Параметры:
//   - method: имя метода для выполнения
//   - params: параметры метода (не используются в текущей реализации)
//
// Возвращает результат выполнения метода или nil, если метод не найден.
func (i *Ice) Exec(method string, params []interface{}) interface{} {
	switch method {
	case "status":
		return i.Status()
	case "getInfo":
		return map[string]interface{}{
			"status":         i.status,
			"started":        i.started,
			"address":        i.address.Hex(),
			"ip":             i.ip,
			"port":           i.port,
			"network_addr":   i.GetNetworkAddr(),
			"bootstrap_ip":   i.bootstrapIP,
			"bootstrap_port": i.bootstrapPort,
			"bootstrap_addr": i.GetBootstrapAddr(),
			"is_bootstrap":   i.isBootstrapNode(),
		}
	case "getAddress":
		return i.address.Hex()
	case "getIP":
		return i.ip
	case "getPort":
		return i.port
	case "getNetworkAddr":
		return i.GetNetworkAddr()
	case "getBootstrapAddr":
		return i.GetBootstrapAddr()
	case "isBootstrap":
		return i.isBootstrapNode()
	case "isBootstrapReady":
		return i.IsBootstrapReady()
	case "waitForBootstrapReady":
		i.WaitForBootstrapReady()
		return true
	case "isConsensusStarted":
		return i.IsConsensusStarted()
	case "waitForConsensus":
		i.WaitForConsensus()
		return true
	default:
		return nil
	}
}
