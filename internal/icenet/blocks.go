package icenet

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/service"
)

// monitorAndSendBlocks периодически проверяет новые блоки и отправляет их на bootstrap
func (i *Ice) monitorAndSendBlocks() {
	ticker := time.NewTicker(5 * time.Second) // Проверяем каждые 5 секунд
	defer ticker.Stop()

	for {
		select {
		case <-i.ctx.Done():
			return
		case <-ticker.C:
			i.mu.Lock()
			started := i.started
			i.mu.Unlock()

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
			i.mu.Lock()
			if latestBlock.Hash == i.lastSentBlockHash {
				i.mu.Unlock()
				continue
			}
			i.mu.Unlock()

			// Отправляем блок на bootstrap
			if err := i.sendBlockToBootstrap(latestBlock); err != nil {
				icelogger().Errorw("Failed to send block to bootstrap",
					"block_hash", latestBlock.Hash.Hex(),
					"err", err,
				)
			} else {
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
}

// sendBlockToBootstrap отправляет блок на bootstrap узел через постоянное соединение
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

	// Сериализуем блок в JSON
	blockJSON, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	// Формируем сообщение: BLOCK|<json_data>\n
	message := fmt.Sprintf("BLOCK|%s\n", string(blockJSON))

	// Отправляем блок через постоянное соединение
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write([]byte(message)); err != nil {
		icelogger().Errorw("Failed to send block to bootstrap", "err", err)
		// Соединение разорвано, оно будет переподключено в connectToBootstrap
		i.mu.Lock()
		if i.bootstrapConn == conn {
			i.bootstrapConn = nil
		}
		i.mu.Unlock()
		return fmt.Errorf("failed to send block: %w", err)
	}

	return nil
}

// Exec выполняет методы сервиса
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
