package icenet

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/cerera/internal/cerera/types"
)

// Start запускает компонент Ice
func (i *Ice) Start() error {
	if i.started {
		icelogger().Warnw("Ice already started")
		return nil
	}

	i.mu.Lock()
	i.started = true
	i.status = 0x1
	i.mu.Unlock()

	// Запускаем listener для входящих подключений
	if err := i.startListener(); err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	// Если мы bootstrap узел, сразу помечаем как готовый и начинаем консенсус
	if i.isBootstrapNode() {
		i.setBootstrapReady()
		i.startConsensus()
		icelogger().Infow("Bootstrap node - marked as ready and consensus started immediately")
	} else {
		// Подключаемся к bootstrap узлу
		go i.connectToBootstrap()
	}

	// while

	// Запускаем мониторинг блоков для отправки на bootstrap
	go i.monitorAndSendBlocks()

	icelogger().Infow("Ice component started",
		"address", i.address.Hex(),
		"network_addr", i.GetNetworkAddr(),
		"bootstrap", i.GetBootstrapAddr(),
	)
	return nil
}

// Stop останавливает компонент Ice
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
	}

	// Закрываем соединение с bootstrap
	i.mu.Lock()
	if i.bootstrapConn != nil {
		if err := i.bootstrapConn.Close(); err != nil {
			icelogger().Errorw("Error closing bootstrap connection", "err", err)
		}
		i.bootstrapConn = nil
	}
	i.mu.Unlock()

	icelogger().Infow("Ice component stopped")
	return nil
}

// Close закрывает компонент Ice
func (i *Ice) Close() error {
	return i.Stop()
}

// Status возвращает статус компонента
func (i *Ice) Status() byte {
	return i.status
}

// ServiceName возвращает имя сервиса для регистрации
func (i *Ice) ServiceName() string {
	return ICE_SERVICE_NAME
}

// GetAddress возвращает адрес cerera
func (i *Ice) GetAddress() types.Address {
	return i.address
}

// GetIP возвращает IP адрес узла
func (i *Ice) GetIP() string {
	return i.ip
}

// GetPort возвращает порт узла
func (i *Ice) GetPort() string {
	return i.port
}

// GetNetworkAddr возвращает полный сетевой адрес в формате "ip:port"
func (i *Ice) GetNetworkAddr() string {
	return fmt.Sprintf("%s:%s", i.ip, i.port)
}

// GetBootstrapAddr возвращает адрес bootstrap узла
func (i *Ice) GetBootstrapAddr() string {
	return fmt.Sprintf("%s:%s", i.bootstrapIP, i.bootstrapPort)
}

// IsBootstrapReady проверяет, готов ли bootstrap (подключен и получен подтверждающий ответ)
func (i *Ice) IsBootstrapReady() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.bootstrapReady
}

// WaitForBootstrapReady блокирует выполнение до тех пор, пока bootstrap не будет готов
func (i *Ice) WaitForBootstrapReady() {
	if i.isBootstrapNode() {
		// Если мы сами bootstrap, сразу готовы
		return
	}

	// Проверяем, может уже готов
	if i.IsBootstrapReady() {
		return
	}

	// Ждем сигнала о готовности
	<-i.readyChan
}

// setBootstrapReady устанавливает флаг готовности bootstrap
func (i *Ice) setBootstrapReady() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.bootstrapReady {
		i.bootstrapReady = true
		close(i.readyChan)
		icelogger().Infow("Bootstrap is ready - validator can proceed")
	}
}

// resetBootstrapReady сбрасывает флаг готовности (при разрыве соединения)
func (i *Ice) resetBootstrapReady() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.bootstrapReady {
		i.bootstrapReady = false
		i.readyChan = make(chan struct{}) // Создаем новый канал
		icelogger().Warnw("Bootstrap ready status reset - validator will wait")
	}
	// Также сбрасываем консенсус при разрыве соединения
	if i.consensusStarted {
		i.consensusStarted = false
		i.consensusChan = make(chan struct{}) // Создаем новый канал
		icelogger().Warnw("Consensus status reset - blocks will not be created")
	}
}

// IsConsensusStarted проверяет, начался ли консенсус
func (i *Ice) IsConsensusStarted() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.consensusStarted
}

// WaitForConsensus блокирует выполнение до начала консенсуса
func (i *Ice) WaitForConsensus() {
	if i.isBootstrapNode() {
		// Если мы сами bootstrap, консенсус может начаться сразу
		return
	}

	// Проверяем, может уже начался
	if i.IsConsensusStarted() {
		return
	}

	// Ждем сигнала о начале консенсуса
	<-i.consensusChan
}

// startConsensus устанавливает флаг начала консенсуса
func (i *Ice) startConsensus() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.consensusStarted {
		i.consensusStarted = true
		close(i.consensusChan)
		icelogger().Infow("Consensus started - blocks can now be created")
	}
}

// isBootstrapNode проверяет, является ли текущий узел bootstrap узлом
func (i *Ice) isBootstrapNode() bool {
	// Bootstrap узел определяется по порту - если порт совпадает с bootstrap портом
	// и узел слушает входящие подключения (не подключается к bootstrap)
	return i.port == i.bootstrapPort
}

// startListener запускает listener для входящих подключений
func (i *Ice) startListener() error {
	addr := fmt.Sprintf(":%s", i.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start listener on %s: %w", addr, err)
	}

	i.mu.Lock()
	i.listener = listener
	i.mu.Unlock()

	icelogger().Infow("Ice listener started", "addr", addr)

	// Обрабатываем входящие подключения
	go i.handleConnections()

	return nil
}

// handleConnections обрабатывает входящие подключения
func (i *Ice) handleConnections() {
	for {
		i.mu.Lock()
		listener := i.listener
		started := i.started
		i.mu.Unlock()

		if !started || listener == nil {
			return
		}

		conn, err := listener.Accept()
		if err != nil {
			i.mu.Lock()
			started = i.started
			i.mu.Unlock()
			if !started {
				return
			}
			icelogger().Errorw("Error accepting connection", "err", err)
			continue
		}

		// Обрабатываем каждое подключение в отдельной горутине
		go i.handleConnection(conn)
	}
}

// handleConnection обрабатывает отдельное подключение
func (i *Ice) handleConnection(conn net.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	icelogger().Infow("New connection", "remote_addr", remoteAddr)

	// При закрытии соединения удаляем его из списка (для bootstrap узла)
	defer func() {
		if i.isBootstrapNode() {
			i.mu.Lock()
			// Находим и удаляем соединение по адресу
			for addr, storedConn := range i.connectedNodes {
				if storedConn == conn {
					delete(i.connectedNodes, addr)
					delete(i.confirmedNodes, addr) // Также удаляем из confirmedNodes
					icelogger().Infow("Removed connection for node", "node_address", addr.Hex())
					break
				}
			}
			i.mu.Unlock()
		}
		conn.Close()
	}()

	// Читаем данные от подключения
	buffer := make([]byte, 1024)
	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				icelogger().Infow("Connection closed by peer", "remote_addr", remoteAddr)
				return
			}
			icelogger().Errorw("Error reading from connection", "remote_addr", remoteAddr, "err", err)
			return
		}

		if n > 0 {
			data := string(buffer[:n])
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

				// Обрабатываем WHO_IS запрос (только для bootstrap узла)
				if i.isWhoIsRequest(line) {
					icelogger().Infow("Received WHO_IS request", "remote_addr", remoteAddr, "data", line)
					i.handleWhoIsRequest(conn, line, remoteAddr)
					continue
				}

				// Обрабатываем NODE_OK (только для bootstrap узла)
				if i.isNodeOkMessage(line) {
					icelogger().Debugw("Received NODE_OK", "remote_addr", remoteAddr, "data", line)
					i.handleNodeOk(conn, line, remoteAddr)
					continue
				}

				// WHO_IS_RESPONSE и CONSENSUS_STATUS обрабатываются только в readFromBootstrap
				// для обычных узлов, которые получают эти сообщения от bootstrap.
				// Bootstrap узел не должен получать эти сообщения через handleConnection.
				// Обычные узлы не должны получать эти сообщения от других узлов (только от bootstrap).
			}
		}
	}
}
