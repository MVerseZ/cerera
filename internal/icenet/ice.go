package icenet

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/protocol"
)

// Start запускает компонент Ice.
// Инициализирует listener для входящих подключений и запускает мониторинг блоков.
// Возвращает ошибку, если не удалось запустить listener.
func (i *Ice) Start() error {
	if i.started {
		icelogger().Warnw("Ice already started")
		return nil
	}

	i.mu.Lock()
	i.started = true
	i.status = 0x1
	i.mu.Unlock()

	// Запускаем listener для входящих подключений через ConnectionManager
	if err := i.startListener(); err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	// Запускаем mesh network (discovery, gossip, connection management)
	if err := i.meshNetwork.Start(); err != nil {
		return fmt.Errorf("failed to start mesh network: %w", err)
	}

	// Подключаемся к seed nodes для первоначального обнаружения пиров
	if i.seedDiscovery != nil {
		go func() {
			if err := i.seedDiscovery.ConnectToSeedNodes(); err != nil {
				icelogger().Warnw("Failed to connect to seed nodes", "err", err)
			} else {
				icelogger().Infow("Connected to seed nodes successfully")
			}
		}()
	}

	// Помечаем сеть как готовую и начинаем консенсус
	i.setNetworkReady()
	i.startConsensus()

	// Запускаем мониторинг блоков для рассылки
	go i.monitorAndSendBlocks()

	icelogger().Infow("Ice component started",
		"address", i.address.Hex(),
		"network_addr", i.GetNetworkAddr(),
	)
	return nil
}

// Stop останавливает компонент Ice.
// Закрывает все соединения, listener и освобождает ресурсы.
// Безопасно вызывать несколько раз - проверяет состояние перед остановкой.
func (i *Ice) Stop() error {
	i.mu.Lock()
	if !i.started {
		i.mu.Unlock()
		icelogger().Warnw("Ice already stopped")
		return nil
	}
	i.started = false
	i.status = 0x0
	i.mu.Unlock()

	// Закрываем listener
	if i.listener != nil {
		if err := i.listener.Close(); err != nil {
			icelogger().Errorw("Error closing listener", "err", err)
		}
		i.listener = nil
	}

	// Останавливаем mesh network
	if i.meshNetwork != nil {
		i.meshNetwork.Stop()
	}

	// Закрываем все соединения через connection manager
	if i.connManager != nil {
		i.connManager.Stop()
	}

	icelogger().Infow("Ice component stopped")
	return nil
}

// Close закрывает компонент Ice.
// Является алиасом для Stop() для совместимости с интерфейсами.
func (i *Ice) Close() error {
	return i.Stop()
}

// Status возвращает текущий статус компонента Ice.
// 0x0 - остановлен, 0x1 - запущен.
func (i *Ice) Status() byte {
	return i.status
}

// ServiceName возвращает имя сервиса для регистрации в service registry.
// Используется для идентификации компонента в системе сервисов.
func (i *Ice) ServiceName() string {
	return ICE_SERVICE_NAME
}

// GetAddress возвращает Cerera адрес текущего узла.
// Адрес используется для идентификации узла в сети.
func (i *Ice) GetAddress() types.Address {
	return i.address
}

// GetIP возвращает IP адрес текущего узла.
// IP адрес определяется автоматически при создании компонента.
func (i *Ice) GetIP() string {
	return i.ip
}

// GetPort возвращает порт, на котором узел слушает входящие подключения.
func (i *Ice) GetPort() string {
	return i.port
}

// GetNetworkAddr возвращает полный сетевой адрес узла в формате "ip:port".
// Используется для сетевых операций и идентификации узла в P2P сети.
func (i *Ice) GetNetworkAddr() string {
	return fmt.Sprintf("%s:%s", i.ip, i.port)
}

// IsNetworkReady проверяет, готова ли сеть.
// Возвращает true, если сеть готова к работе.
func (i *Ice) IsNetworkReady() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.networkReady
}

// WaitForNetworkReady блокирует выполнение до тех пор, пока сеть не будет готова.
// Используется для синхронизации перед началом работы валидатора.
func (i *Ice) WaitForNetworkReady() {
	// Проверяем, может уже готов
	if i.IsNetworkReady() {
		return
	}

	// Безопасно получаем канал под блокировкой
	i.mu.RLock()
	readyChan := i.readyChan
	i.mu.RUnlock()

	// Ждем сигнала о готовности
	<-readyChan
}

// setNetworkReady устанавливает флаг готовности сети
func (i *Ice) setNetworkReady() {
	i.mu.Lock()
	wasReady := i.networkReady
	i.networkReady = true
	readyChan := i.readyChan
	i.mu.Unlock()

	if !wasReady {
		// Безопасно закрываем канал только один раз
		i.readyOnce.Do(func() {
			close(readyChan)
			icelogger().Infow("Network is ready - validator can proceed")
		})
	}
}

// resetNetworkReady сбрасывает флаг готовности сети
func (i *Ice) resetNetworkReady() {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.networkReady {
		i.networkReady = false
		// Создаем новый канал и новый sync.Once для следующего цикла
		i.readyChan = make(chan struct{})
		i.readyOnce = sync.Once{}
		icelogger().Warnw("Network ready status reset - validator will wait")
	}

	// Также сбрасываем консенсус при разрыве соединения
	if i.consensusStarted {
		i.consensusStarted = false
		// Создаем новый канал и новый sync.Once для следующего цикла
		i.consensusChan = make(chan struct{})
		i.consensusOnce = sync.Once{}
		icelogger().Warnw("Consensus status reset - blocks will not be created")
	}
}

// IsConsensusStarted проверяет, начался ли процесс консенсуса.
// Возвращает true, если консенсус активен и блоки могут создаваться.
func (i *Ice) IsConsensusStarted() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.consensusStarted
}

// WaitForConsensus блокирует выполнение до начала консенсуса.
// Используется для синхронизации перед созданием блоков.
func (i *Ice) WaitForConsensus() {
	// Проверяем, может уже начался
	if i.IsConsensusStarted() {
		return
	}

	// Безопасно получаем канал под блокировкой
	i.mu.RLock()
	consensusChan := i.consensusChan
	i.mu.RUnlock()

	// Ждем сигнала о начале консенсуса
	<-consensusChan
}

// startConsensus устанавливает флаг начала консенсуса
func (i *Ice) startConsensus() {
	i.mu.Lock()
	wasStarted := i.consensusStarted
	i.consensusStarted = true
	consensusChan := i.consensusChan
	i.mu.Unlock()

	if !wasStarted {
		// Безопасно закрываем канал только один раз
		i.consensusOnce.Do(func() {
			close(consensusChan)
			icelogger().Infow("Consensus started - blocks can now be created")
		})
	}
}


// startListener запускает listener для входящих подключений
func (i *Ice) startListener() error {
	// Use connection manager instead of direct listener
	if err := i.connManager.Start(i.port); err != nil {
		return fmt.Errorf("failed to start connection manager: %w", err)
	}

	icelogger().Infow("Ice listener started via connection manager", "port", i.port)

	// Process messages from connection handler
	go i.processConnectionMessages()

	return nil
}

// processConnectionMessages processes messages from the connection handler
func (i *Ice) processConnectionMessages() {
	handler := i.connManager.GetHandler()
	msgChan := handler.MessageChannel()

	for {
		select {
		case <-i.ctx.Done():
			icelogger().Infow("Message processor stopped due to context cancellation")
			return
		case event := <-msgChan:
			if event.Error != nil {
				icelogger().Errorw("Message event error", "err", event.Error)
				continue
			}

			if event.Message == nil {
				continue
			}

			// Process domain-specific messages
			i.processProtocolMessage(event.Message, event.Connection)
		}
	}
}

// processProtocolMessage processes a protocol message (domain-specific logic)
func (i *Ice) processProtocolMessage(msg protocol.Message, conn *connection.Connection) {
	// This replaces the old handleConnection logic
	// Domain-specific message handling is preserved
	if conn == nil || conn.Conn == nil {
		return
	}

	switch m := msg.(type) {
	case *protocol.ReadyRequestMessage:
		// Convert to old format for compatibility
		data := fmt.Sprintf("%s|%s|%s", protocol.MsgTypeReadyRequest, m.Address.Hex(), m.NetworkAddr)
		i.handleReadyRequest(conn.Conn, data, conn.NetworkAddr)
	case *protocol.WhoIsMessage:
		data := fmt.Sprintf("%s|%s", protocol.MsgTypeWhoIs, m.NodeAddress.Hex())
		i.handleWhoIsRequest(conn.Conn, data, conn.NetworkAddr)
	case *protocol.NodeOkMessage:
		data := fmt.Sprintf("%s|%d|%d", protocol.MsgTypeNodeOk, m.Count, m.Nonce)
		i.handleNodeOk(conn.Conn, data, conn.NetworkAddr)
	case *protocol.BlockMessage:
		// Block messages need special handling
		if m.Block != nil {
			blockJSON, _ := json.Marshal(m.Block)
			data := fmt.Sprintf("%s|%s", protocol.MsgTypeBlock, string(blockJSON))
			i.processReceivedBlock(data, conn.NetworkAddr)
		}
	case *protocol.ProposalMessage:
		i.handleProposalMessage(m)
	case *protocol.VoteMessage:
		i.handleVoteMessage(m)
	case *protocol.ConsensusResultMessage:
		i.handleConsensusResultMessage(m)
	case *protocol.PeerDiscoveryMessage:
		i.handlePeerDiscoveryMessage(m, conn)
		// Add other message types as needed
	}
}

// handleConnection обрабатывает отдельное подключение
func (i *Ice) handleConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	icelogger().Infow("New connection", "remote_addr", remoteAddr)

	// При закрытии соединения закрываем его
	defer func() {
		if err := conn.Close(); err != nil {
			icelogger().Debugw("Error closing connection", "remote_addr", remoteAddr, "err", err)
		}
	}()

	// Check context cancellation in the connection handler
	ctxDone := make(chan struct{})
	go func() {
		<-i.ctx.Done()
		close(ctxDone)
		conn.Close()
	}()

	// Читаем данные от подключения
	buffer := make([]byte, DefaultConnectionBufferSize)
	for {
		// Check context cancellation
		select {
		case <-ctxDone:
			icelogger().Infow("Connection handler cancelled", "remote_addr", remoteAddr)
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(DefaultReadTimeout))
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				icelogger().Infow("Connection closed by peer", "remote_addr", remoteAddr)
				return
			}
			// Check if error is due to context cancellation
			select {
			case <-ctxDone:
				return
			default:
			}
			icelogger().Errorw("Error reading from connection", "remote_addr", remoteAddr, "err", err)
			return
		}

		if n > 0 {
			// Validate message size
			if err := ValidateMessageSize(n); err != nil {
				icelogger().Errorw("Invalid message size", "remote_addr", remoteAddr, "size", n, "err", err)
				return
			}

			data := string(buffer[:n])
			// Sanitize message to prevent injection attacks
			data = SanitizeMessage(data)
			icelogger().Debugw("Received data from connection",
				"remote_addr", remoteAddr,
				"size", n,
				"data", data,
			)

			// Разбиваем данные по строкам для обработки нескольких сообщений
			lines := strings.Split(data, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				// Обрабатываем READY_REQUEST
				if i.isReadyRequest(line) {
					icelogger().Infow("Processing READY_REQUEST", "data", line)
					i.handleReadyRequest(conn, line, remoteAddr)
					continue
				}

				// Обрабатываем WHO_IS запрос
				if i.isWhoIsRequest(line) {
					icelogger().Infow("Received WHO_IS request", "remote_addr", remoteAddr, "data", line)
					i.handleWhoIsRequest(conn, line, remoteAddr)
					continue
				}

				// Обрабатываем NODE_OK
				if i.isNodeOkMessage(line) {
					icelogger().Debugw("Received NODE_OK", "remote_addr", remoteAddr, "data", line)
					i.handleNodeOk(conn, line, remoteAddr)
					continue
				}

				// Обрабатываем BLOCK сообщения
				if i.isBlockMessage(line) {
					icelogger().Infow("Received BLOCK message from node", "remote_addr", remoteAddr)
					i.processReceivedBlock(line, remoteAddr)
					continue
				}
			}
		}
	}
}
