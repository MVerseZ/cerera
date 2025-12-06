package icenet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"
	"github.com/cerera/internal/cerera/service"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
	"go.uber.org/zap"
)

const ICE_SERVICE_NAME = "ICE_CERERA_001_1_0"

const (
	bootstrapIP   = "192.168.1.6"
	bootstrapPort = "31100"
)

// icelogger returns a sugared logger for the icenet package.
// It is defined as a function (not a global variable) so that it always
// uses the logger configured by logger.Init(), even if this package is
// imported before logging is set up in main().
func icelogger() *zap.SugaredLogger {
	return logger.Named("icenet")
}

// Ice представляет основной компонент Ice
type Ice struct {
	cfg               *config.Config
	ctx               context.Context
	status            byte
	started           bool
	address           types.Address              // адрес cerera
	ip                string                     // IP адрес узла
	port              string                     // порт узла
	bootstrapIP       string                     // IP адрес bootstrap узла
	bootstrapPort     string                     // порт bootstrap узла
	listener          net.Listener               // listener для входящих подключений
	bootstrapConn     net.Conn                   // постоянное соединение с bootstrap
	connectedNodes    map[types.Address]net.Conn // активные соединения с узлами (только для bootstrap)
	mu                sync.Mutex                 // мьютекс для потокобезопасности
	lastSentBlockHash common.Hash                // хеш последнего отправленного блока
	bootstrapReady    bool                       // флаг готовности bootstrap соединения
	readyChan         chan struct{}              // канал для уведомления о готовности
	consensusStarted  bool                       // флаг начала консенсуса
	consensusChan     chan struct{}              // канал для уведомления о начале консенсуса
}

// NewIce создаёт новый экземпляр Ice
func NewIce(cfg *config.Config, ctx context.Context, port string) (*Ice, error) {
	// Определяем IP адрес автоматически
	currentIP := ""
	interfaces, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("error getting network interfaces: %w", err)
	}
	for _, iface := range interfaces {
		ip, _, err := net.ParseCIDR(iface.String())
		if err != nil {
			continue
		}
		if ip.To4() != nil {
			currentIP = ip.String()
			break
		}
	}
	if currentIP == "" {
		return nil, fmt.Errorf("no IPv4 interface found")
	}

	ice := &Ice{
		cfg:              cfg,
		ctx:              ctx,
		status:           0x0,
		started:          false,
		address:          cfg.NetCfg.ADDR, // адрес cerera из конфигурации
		ip:               currentIP,
		port:             port,
		bootstrapIP:      bootstrapIP,
		bootstrapPort:    bootstrapPort,
		bootstrapReady:   false,
		readyChan:        make(chan struct{}),
		consensusStarted: false,
		consensusChan:    make(chan struct{}),
		connectedNodes:   make(map[types.Address]net.Conn), // инициализируем map для хранения соединений
	}

	icelogger().Infow("Ice component created",
		"address", cfg.NetCfg.ADDR.Hex(),
		"ip", currentIP,
		"port", port,
		"bootstrap", fmt.Sprintf("%s:%s", bootstrapIP, bootstrapPort),
	)
	return ice, nil
}

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
	return i.GetNetworkAddr() == i.GetBootstrapAddr()
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
			icelogger().Infow("Received data from connection",
				"remote_addr", remoteAddr,
				"size", n,
				"data", data,
			)

			// Обрабатываем READY_REQUEST
			if i.isReadyRequest(data) {
				icelogger().Infow("Processing READY_REQUEST", "data", data)
				i.handleReadyRequest(conn, data, remoteAddr)
			}
		}
	}
}

// isReadyRequest проверяет, является ли сообщение запросом на включение
func (i *Ice) isReadyRequest(data string) bool {
	// Формат: READY_REQUEST|{address}|{network_addr}
	return len(data) > 12 && strings.HasPrefix(data, "READY_REQUEST")
}

// handleReadyRequest обрабатывает запрос на включение и отправляет ответ
func (i *Ice) handleReadyRequest(conn net.Conn, data string, remoteAddr string) {
	// Формат: READY_REQUEST|{address}|{network_addr}
	parts := i.splitMessage(data, "|")
	if len(parts) < 3 {
		icelogger().Warnw("Invalid READY_REQUEST format", "data", data)
		return
	}

	nodeAddressStr := i.trimResponse(parts[1])
	networkAddr := i.trimResponse(parts[2])

	// Парсим адрес узла
	nodeAddr := i.parseAddress(nodeAddressStr)
	if nodeAddr == nil {
		icelogger().Warnw("Failed to parse node address from READY_REQUEST", "addr", nodeAddressStr)
		return
	}

	icelogger().Infow("Processing READY_REQUEST",
		"remote_addr", remoteAddr,
		"node_address", nodeAddr.Hex(),
		"network_addr", networkAddr,
	)

	// Добавляем узел в консенсус
	gigea.AddVoter(*nodeAddr)
	gigea.AddNode(*nodeAddr, networkAddr)

	// Отправляем ответ READY
	readyResponse := "READY\n"
	if _, err := conn.Write([]byte(readyResponse)); err != nil {
		icelogger().Errorw("Error sending READY response", "remote_addr", remoteAddr, "err", err)
		return
	}

	icelogger().Infow("Sent READY response", "remote_addr", remoteAddr)

	// Отправляем список узлов
	i.sendNodeList(conn, remoteAddr)

	// Отправляем команду начала консенсуса
	consensusStartCmd := "START_CONSENSUS\n"
	if _, err := conn.Write([]byte(consensusStartCmd)); err != nil {
		icelogger().Errorw("Error sending START_CONSENSUS", "remote_addr", remoteAddr, "err", err)
		return
	}

	icelogger().Infow("Sent START_CONSENSUS command", "remote_addr", remoteAddr)

	// Отправляем текущий nonce для синхронизации
	currentNonce := gigea.GetNonce()
	nonceMessage := fmt.Sprintf("NONCE|%d\n", currentNonce)
	if _, err := conn.Write([]byte(nonceMessage)); err != nil {
		icelogger().Errorw("Error sending nonce", "remote_addr", remoteAddr, "err", err)
		return
	}

	icelogger().Infow("Sent nonce to new node",
		"remote_addr", remoteAddr,
		"nonce", currentNonce,
	)

	// Сохраняем соединение для bootstrap узла
	if i.isBootstrapNode() {
		i.mu.Lock()
		i.connectedNodes[*nodeAddr] = conn
		i.mu.Unlock()
		icelogger().Infow("Saved connection for node", "node_address", nodeAddr.Hex())
	}

	// Рассылаем статус консенсуса всем подключенным узлам
	if i.isBootstrapNode() {
		i.broadcastConsensusStatus()
	}
}

// sendNodeList отправляет список узлов подключенному узлу
func (i *Ice) sendNodeList(conn net.Conn, remoteAddr string) {
	nodes := gigea.GetNodes()
	if len(nodes) == 0 {
		return
	}

	// Формируем список узлов: address1#network_addr1,address2#network_addr2,...
	var nodeList []string
	for addr, node := range nodes {
		if node.IsConnected {
			nodeList = append(nodeList, fmt.Sprintf("%s#%s", addr, node.NetworkAddr))
		}
	}

	if len(nodeList) > 0 {
		nodeListStr := ""
		for idx, nodeInfo := range nodeList {
			if idx > 0 {
				nodeListStr += ","
			}
			nodeListStr += nodeInfo
		}

		message := fmt.Sprintf("NODES|%s\n", nodeListStr)
		if _, err := conn.Write([]byte(message)); err != nil {
			icelogger().Errorw("Error sending node list", "remote_addr", remoteAddr, "err", err)
			return
		}

		icelogger().Infow("Sent node list to new node",
			"remote_addr", remoteAddr,
			"nodes_count", len(nodeList),
		)
	}
}

// splitMessage разбивает сообщение по разделителю
func (i *Ice) splitMessage(msg, delimiter string) []string {
	var parts []string
	current := ""
	for _, char := range msg {
		if string(char) == delimiter {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else if char != '\n' && char != '\r' {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// trimResponse убирает пробелы и переносы строк из ответа
func (i *Ice) trimResponse(s string) string {
	result := ""
	for _, char := range s {
		if char != ' ' && char != '\n' && char != '\r' && char != '\t' {
			result += string(char)
		}
	}
	return result
}

// broadcastConsensusStatus рассылает статус консенсуса всем подключенным узлам
func (i *Ice) broadcastConsensusStatus() {
	if !i.isBootstrapNode() {
		return
	}

	// Получаем информацию о консенсусе
	consensusInfo := gigea.GetConsensusInfo()

	// Формируем сообщение со статусом консенсуса
	// Формат: CONSENSUS_STATUS|{status}|{voters_count}|{nodes_count}|{nonce}
	statusMessage := fmt.Sprintf("CONSENSUS_STATUS|%v|%v|%v|%v\n",
		consensusInfo["status"],
		consensusInfo["voters"],
		consensusInfo["nodes"],
		consensusInfo["nonce"],
	)

	i.mu.Lock()
	// Копируем список соединений для безопасной итерации
	connections := make(map[types.Address]net.Conn)
	for addr, conn := range i.connectedNodes {
		connections[addr] = conn
	}
	i.mu.Unlock()

	// Рассылаем статус всем подключенным узлам
	sentCount := 0
	for addr, conn := range connections {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write([]byte(statusMessage)); err != nil {
			icelogger().Warnw("Failed to send consensus status to node",
				"node_address", addr.Hex(),
				"err", err,
			)
			// Удаляем неработающее соединение
			i.mu.Lock()
			delete(i.connectedNodes, addr)
			i.mu.Unlock()
		} else {
			sentCount++
			icelogger().Debugw("Sent consensus status to node",
				"node_address", addr.Hex(),
			)
		}
	}

	if sentCount > 0 {
		icelogger().Infow("Broadcasted consensus status to connected nodes",
			"nodes_count", sentCount,
			"status", consensusInfo["status"],
			"voters", consensusInfo["voters"],
			"nodes", consensusInfo["nodes"],
			"nonce", consensusInfo["nonce"],
		)
	}
}

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
