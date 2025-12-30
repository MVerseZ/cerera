package icenet

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
)

// connectToBootstrap подключается к bootstrap узлу и поддерживает соединение постоянно
// Используется только обычными узлами (не bootstrap)
func (i *Ice) connectToBootstrap() {
	bootstrapAddr := i.GetBootstrapAddr()
	baseDelay := 3 * time.Second
	maxDelay := 60 * time.Second
	maxRetries := 10
	retries := 0

	for {
		// Check context cancellation
		select {
		case <-i.ctx.Done():
			icelogger().Infow("Bootstrap connection cancelled due to context")
			return
		default:
		}

		i.mu.RLock()
		started := i.started
		i.mu.RUnlock()

		if !started {
			return
		}

		icelogger().Debugw("Connecting to bootstrap node", "bootstrap_addr", bootstrapAddr, "attempt", retries+1)

		// Use context-aware dial with timeout
		dialer := &net.Dialer{
			Timeout: 10 * time.Second,
		}
		conn, err := dialer.DialContext(i.ctx, "tcp", bootstrapAddr)
		if err != nil {
			retries++
			if retries >= maxRetries {
				icelogger().Errorw("Failed to connect to bootstrap after max retries",
					"bootstrap_addr", bootstrapAddr,
					"retries", retries,
					"max_retries", maxRetries,
					"err", err)
				break
			}

			// Calculate exponential backoff
			backoffDelay := CalculateBackoff(retries-1, baseDelay, maxDelay)
			icelogger().Warnw("Failed to connect to bootstrap, retrying",
				"bootstrap_addr", bootstrapAddr,
				"attempt", retries,
				"max_retries", maxRetries,
				"backoff_delay", backoffDelay,
				"err", err)

			// Wait with context cancellation support
			select {
			case <-i.ctx.Done():
				return
			case <-time.After(backoffDelay):
				// Continue retry
			}
			continue
		}

		// Reset retry counter on successful connection
		retries = 0

		icelogger().Infow("Successfully connected to bootstrap", "bootstrap_addr", bootstrapAddr)

		// Сохраняем соединение
		i.mu.Lock()
		i.bootstrapConn = conn
		i.mu.Unlock()

		// Добавляем текущий узел в консенсус
		i.recalculateConsensus()

		// Отправляем запрос на включение с информацией о себе
		readyRequest := fmt.Sprintf("%s|%s|%s\n", MsgTypeReadyRequest, i.address.Hex(), i.GetNetworkAddr())
		conn.SetWriteDeadline(time.Now().Add(DefaultConnectionTimeout))
		if _, err := conn.Write([]byte(readyRequest)); err != nil {
			icelogger().Errorw("Error sending ready request to bootstrap", "err", err)
			conn.Close()
			i.mu.Lock()
			i.bootstrapConn = nil
			i.mu.Unlock()

			// Wait before retry with context support
			backoffDelay := CalculateBackoff(0, baseDelay, maxDelay)
			select {
			case <-i.ctx.Done():
				return
			case <-time.After(backoffDelay):
				// Continue retry
			}
			continue
		}

		// Запускаем горутину для чтения ответов от bootstrap
		go i.readFromBootstrap(conn)

		// Отправляем keep-alive пакеты периодически
		go i.sendKeepAlive(conn)

		// Отправляем периодические подтверждения NODE_OK
		go i.sendPeriodicNodeOk(conn)

		// Ждем разрыва соединения
		// Проверяем соединение периодически
		ticker := time.NewTicker(DefaultPingInterval)
		defer ticker.Stop()

		connectionAlive := true
		for connectionAlive {
			select {
			case <-ticker.C:
				// Проверяем, что соединение еще живо
				conn.SetWriteDeadline(time.Now().Add(DefaultPingTimeout))
				pingMsg := fmt.Sprintf("%s\n", MsgTypePing)
				if _, err := conn.Write([]byte(pingMsg)); err != nil {
					icelogger().Warnw("Bootstrap connection lost, reconnecting", "err", err)
					conn.Close()
					i.mu.Lock()
					i.bootstrapConn = nil
					i.mu.Unlock()
					i.resetBootstrapReady()
					connectionAlive = false
				}
			case <-i.ctx.Done():
				icelogger().Infow("Bootstrap connection cancelled, closing")
				conn.Close()
				i.mu.Lock()
				i.bootstrapConn = nil
				i.mu.Unlock()
				return
			}
		}

		// Если соединение разорвано, ждем перед переподключением с экспоненциальной задержкой
		backoffDelay := CalculateBackoff(0, baseDelay, maxDelay)
		select {
		case <-i.ctx.Done():
			return
		case <-time.After(backoffDelay):
			// Continue to reconnect
		}
	}
}

// readFromBootstrap читает данные от bootstrap соединения
// Используется только обычными узлами (не bootstrap)
func (i *Ice) readFromBootstrap(conn net.Conn) {
	buffer := make([]byte, DefaultReadBufferSize)
	for {
		conn.SetReadDeadline(time.Now().Add(DefaultBootstrapReadTimeout))
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				icelogger().Infow("Bootstrap connection closed by peer")
				i.resetBootstrapReady()
			} else {
				icelogger().Errorw("Error reading from bootstrap", "err", err)
				i.resetBootstrapReady()
			}
			return
		}

		if n > 0 {
			// Validate message size
			if err := ValidateMessageSize(n); err != nil {
				icelogger().Errorw("Invalid message size from bootstrap", "size", n, "err", err)
				return
			}

			data := string(buffer[:n])
			// Sanitize message to prevent injection attacks
			data = SanitizeMessage(data)
			icelogger().Infow("Received data from bootstrap",
				"size", n,
				"data", data,
			)

			// Проверяем, является ли это структурированным сообщением REQ
			// REQ содержит READY, NODES и NONCE в одном сообщении
			// REQ - многострочное сообщение, обрабатываем весь буфер
			if i.isREQMessage(data) {
				i.processREQMessage(data)
				continue
			}

			// Разбиваем данные по строкам для обработки нескольких сообщений
			lines := strings.Split(data, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				// Проверяем, является ли это командой начала консенсуса
				if i.isConsensusStartCommand(line) {
					i.startConsensus()
					continue
				}

				// Проверяем, содержит ли сообщение COUNT узлов (для broadcast сообщений)
				if i.isNodesCountMessage(line) {
					i.processNodesCountMessage(line)
					continue
				}

				// Проверяем, содержит ли сообщение список узлов (для broadcast сообщений)
				if i.isNodeListMessage(line) {
					i.processNodeListMessage(line)
					continue
				}

				// Проверяем, содержит ли сообщение broadcast nonce
				if i.isBroadcastNonceMessage(line) {
					i.processBroadcastNonceMessage(line)
					continue
				}

				// Проверяем, содержит ли сообщение WHO_IS ответ
				if i.isWhoIsResponse(line) {
					i.processWhoIsResponse(line)
					continue
				}

				// Проверяем, содержит ли сообщение статус консенсуса
				if i.isConsensusStatusMessage(line) {
					i.processConsensusStatusMessage(line)
					continue
				}

				// Проверяем, является ли сообщение блоком
				if i.isBlockMessage(line) {
					icelogger().Infow("Received BLOCK message from bootstrap")
					i.processReceivedBlock(line, "bootstrap")
					continue
				}
			}
		}
	}
}

// sendKeepAlive отправляет keep-alive пакеты для поддержания соединения
// Используется только обычными узлами (не bootstrap)
func (i *Ice) sendKeepAlive(conn net.Conn) {
	ticker := time.NewTicker(DefaultKeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			i.mu.RLock()
			started := i.started
			currentConn := i.bootstrapConn
			i.mu.RUnlock()

			if !started || currentConn != conn {
				return
			}

			keepAliveMsg := fmt.Sprintf("%s\n", MsgTypeKeepAlive)
			conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout))
			if _, err := conn.Write([]byte(keepAliveMsg)); err != nil {
				icelogger().Warnw("Failed to send keep-alive", "err", err)
				return
			}
		case <-i.ctx.Done():
			return
		}
	}
}

// sendPeriodicNodeOk периодически отправляет NODE_OK на bootstrap для подтверждения
// Используется только обычными узлами (не bootstrap)
func (i *Ice) sendPeriodicNodeOk(conn net.Conn) {
	// Отправляем первое подтверждение сразу после подключения (уже отправляется в processNodesCountMessage)
	// Затем отправляем периодически с интервалом
	ticker := time.NewTicker(DefaultNodeOkInterval)
	defer ticker.Stop()

	// Пропускаем первый тик, так как первое подтверждение уже отправлено
	<-ticker.C

	for {
		select {
		case <-ticker.C:
			i.mu.RLock()
			started := i.started
			currentConn := i.bootstrapConn
			bootstrapReady := i.bootstrapReady
			i.mu.RUnlock()

			if !started || currentConn != conn || !bootstrapReady {
				continue
			}

			// Получаем текущее количество узлов и nonce
			i.mu.RLock()
			consensusManager := i.consensusManager
			i.mu.RUnlock()
			nodes := consensusManager.GetNodes()
			actualCount := len(nodes)
			currentNonce := consensusManager.GetNonce()

			// Логируем текущий nonce перед отправкой
			icelogger().Debugw("Preparing to send periodic NODE_OK",
				"node_count", actualCount,
				"current_nonce", currentNonce,
			)

			// Отправляем периодическое подтверждение NODE_OK
			message := fmt.Sprintf("%s|%d|%d\n", MsgTypeNodeOk, actualCount, currentNonce)
			conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout))
			if _, err := conn.Write([]byte(message)); err != nil {
				icelogger().Warnw("Failed to send periodic NODE_OK to bootstrap", "err", err)
				return
			}

			icelogger().Debugw("Sent periodic NODE_OK to bootstrap",
				"node_count", actualCount,
				"nonce", currentNonce,
			)
		case <-i.ctx.Done():
			return
		}
	}
}

// processREQMessage обрабатывает структурированное сообщение REQ
// Используется только обычными узлами (не bootstrap)
func (i *Ice) processREQMessage(data string) {
	req, err := parseREQ(data)
	if err != nil {
		icelogger().Errorw("Failed to parse REQ message", "err", err, "data", data)
		return
	}

	icelogger().Infow("Processing REQ message",
		"bootstrap_address", req.Address.Hex(),
		"bootstrap_network_addr", req.NetworkAddr,
		"nodes_count", len(req.Nodes),
		"nonce", req.Nonce,
	)

	// Устанавливаем флаг готовности (это заменяет READY)
	i.setBootstrapReady()

	// Обрабатываем список узлов (это заменяет NODES) - используем общую функцию
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	addedCount, err := addNodesFromList(req.Nodes, &i.address, consensusManager)
	if err != nil {
		icelogger().Warnw("Error adding nodes from REQ", "err", err)
	}

	if addedCount > 0 {
		icelogger().Infow("Processed nodes from REQ",
			"nodes_added", addedCount,
		)

		// Обновляем консенсус после получения списка узлов
		i.updateConsensusAfterNodes()

		// Сверяем узлы со своими и проверяем IP адреса
		i.verifyNodesIPAddresses()
	}

	// Обновляем nonce (это заменяет NONCE)
	// consensusManager уже получен выше, используем его
	currentNonce := consensusManager.GetNonce()
	if req.Nonce != currentNonce {
		icelogger().Infow("Updating consensus nonce from REQ",
			"old_nonce", currentNonce,
			"new_nonce", req.Nonce,
		)
		consensusManager.SetNonce(req.Nonce)
	} else {
		icelogger().Debugw("Nonce already synchronized", "nonce", req.Nonce)
	}

	// Получаем обновленную информацию о консенсусе
	// consensusManager уже получен выше, используем его
	consensusInfo := consensusManager.GetConsensusInfo()
	icelogger().Infow("REQ message processed successfully",
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"status", consensusInfo["status"],
		"nonce", consensusInfo["nonce"],
	)
}

// isNodeListMessage проверяет, содержит ли сообщение список узлов
func (i *Ice) isNodeListMessage(data string) bool {
	// Формат: NODES|address1#network_addr1,address2#network_addr2,...
	// Проверяем, что это именно NODES, а не NODES_COUNT
	return len(data) > len(MsgTypeNodes) && strings.HasPrefix(data, MsgTypeNodes) && !strings.HasPrefix(data, MsgTypeNodesCount)
}

// processNodeListMessage обрабатывает сообщение со списком узлов и добавляет их в консенсус
// Используется только обычными узлами (не bootstrap)
func (i *Ice) processNodeListMessage(data string) {
	// Формат: NODES|address1#network_addr1,address2#network_addr2,...
	parts := splitMessage(data, "|")
	if len(parts) < 2 {
		icelogger().Warnw("Invalid node list message format", "data", data)
		return
	}

	nodeListStr := parts[1]
	nodes := splitMessage(nodeListStr, ",")

	addedCount := 0
	for _, nodeInfo := range nodes {
		nodeParts := splitMessage(nodeInfo, "#")
		if len(nodeParts) == 2 {
			nodeAddrStr := nodeParts[0]
			networkAddr := nodeParts[1]

			// Парсим адрес (используем метод из messages.go)
			nodeAddr := i.parseAddress(nodeAddrStr)
			if nodeAddr == nil {
				icelogger().Warnw("Failed to parse node address", "addr", nodeAddrStr)
				continue
			}

			// Добавляем узел в консенсус (используем общую функцию)
			i.mu.RLock()
			consensusManager := i.consensusManager
			i.mu.RUnlock()
			if err := addNodeToConsensus(*nodeAddr, networkAddr, consensusManager); err != nil {
				icelogger().Warnw("Failed to add node to consensus from node list",
					"address", nodeAddr.Hex(),
					"network_addr", networkAddr,
					"err", err)
				continue
			}
			addedCount++
		}
	}

	if addedCount > 0 {
		icelogger().Infow("Processed node list from bootstrap",
			"nodes_added", addedCount,
		)

		// Обновляем консенсус после получения списка узлов
		i.updateConsensusAfterNodes()

		// Сверяем узлы со своими и проверяем IP адреса
		i.verifyNodesIPAddresses()

		// Получаем обновленную информацию о консенсусе
		consensusInfo := gigea.GetConsensusInfo()
		icelogger().Infow("Updated consensus info",
			"voters", consensusInfo["voters"],
			"nodes", consensusInfo["nodes"],
			"status", consensusInfo["status"],
			"nonce", consensusInfo["nonce"],
		)

		// Не bootstrap узлы не отправляют nonce - они получают его от bootstrap
	}
}

// isNodesCountMessage проверяет, содержит ли сообщение COUNT узлов
func (i *Ice) isNodesCountMessage(data string) bool {
	// Формат: NODES_COUNT|{count}
	return len(data) > len(MsgTypeNodesCount) && strings.HasPrefix(data, MsgTypeNodesCount)
}

// processNodesCountMessage обрабатывает сообщение с COUNT узлов
// Используется только обычными узлами (не bootstrap)
func (i *Ice) processNodesCountMessage(data string) {
	// Формат: NODES_COUNT|{count}
	parts := splitMessage(data, "|")
	if len(parts) < 2 {
		icelogger().Warnw("Invalid NODES_COUNT message format", "data", data)
		return
	}

	countStr := trimResponse(parts[1])
	var expectedCount int
	if _, err := fmt.Sscanf(countStr, "%d", &expectedCount); err != nil {
		icelogger().Warnw("Failed to parse NODES_COUNT", "count_str", countStr, "err", err)
		return
	}

	// Получаем текущее количество узлов в консенсусе
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	nodes := consensusManager.GetNodes()
	actualCount := len(nodes)

	icelogger().Infow("Received NODES_COUNT from bootstrap",
		"expected_count", expectedCount,
		"actual_count", actualCount,
	)

	// Валидация: проверяем, соответствует ли количество узлов
	if actualCount != expectedCount {
		icelogger().Warnw("Node count mismatch",
			"expected_from_bootstrap", expectedCount,
			"actual_in_consensus", actualCount,
		)
	} else {
		icelogger().Infow("Node count validated successfully",
			"count", expectedCount,
		)
		// Отправляем сообщение "NODE_OK" на bootstrap, подтверждая успешную обработку количества узлов
		i.mu.Lock()
		conn := i.bootstrapConn
		i.mu.Unlock()
		if conn != nil {
			// Отправляем подтверждение NODE_OK, количество узлов и nonce в одном пакете формата: NODE_OK|count|nonce
			i.mu.RLock()
			consensusManager := i.consensusManager
			i.mu.RUnlock()
			currentNonce := consensusManager.GetNonce()
			message := fmt.Sprintf("%s|%d|%d\n", MsgTypeNodeOk, actualCount, currentNonce)
			if _, err := conn.Write([]byte(message)); err != nil {
				icelogger().Warnw("Failed to send NODE_OK|count|nonce to bootstrap", "err", err)
			}
		}
	}
}

// isBroadcastNonceMessage проверяет, содержит ли сообщение broadcast nonce
func (i *Ice) isBroadcastNonceMessage(data string) bool {
	// Формат: BROADCAST_NONCE|{nonce_value}|{node_list}
	return len(data) > len(MsgTypeBroadcastNonce) && strings.HasPrefix(data, MsgTypeBroadcastNonce)
}

// processBroadcastNonceMessage обрабатывает сообщение с broadcast nonce
// Используется только обычными узлами (не bootstrap)
func (i *Ice) processBroadcastNonceMessage(data string) {
	// Формат: BROADCAST_NONCE|{nonce_value}|{node_list}
	parts := splitMessage(data, "|")
	if len(parts) < 2 {
		icelogger().Warnw("Invalid broadcast nonce message format", "data", data)
		return
	}

	// Парсим nonce
	nonceStr := trimResponse(parts[1])
	var nonce uint64
	if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
		icelogger().Warnw("Failed to parse broadcast nonce", "nonce_str", nonceStr, "err", err)
		return
	}

	// Обновляем nonce в консенсусе
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	currentNonce := consensusManager.GetNonce()
	if nonce != currentNonce {
		icelogger().Infow("Updating consensus nonce from broadcast",
			"old_nonce", currentNonce,
			"new_nonce", nonce,
		)
		consensusManager.SetNonce(nonce)
	}

	// Если есть список узлов, обрабатываем его
	if len(parts) >= 3 {
		nodeListStr := parts[2]
		// Обрабатываем список узлов (может быть пустым)
		if nodeListStr != "" {
			icelogger().Debugw("Received node list with broadcast nonce", "node_list", nodeListStr)
		}
	}
}

// isConsensusStartCommand проверяет, является ли сообщение командой начала консенсуса
func (i *Ice) isConsensusStartCommand(data string) bool {
	// Проверяем различные варианты команд начала консенсуса
	commands := []string{
		MsgTypeStartConsensus,
		MsgTypeConsensusStart,
		MsgTypeBeginConsensus,
		MsgTypeConsensusBegin,
	}
	data = trimResponse(data)
	for _, cmd := range commands {
		if data == cmd {
			return true
		}
	}
	return false
}

// verifyNodesIPAddresses сверяет узлы со своими и проверяет IP адреса
// Используется только обычными узлами (не bootstrap)
func (i *Ice) verifyNodesIPAddresses() {
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	nodes := consensusManager.GetNodes()

	knownCount := 0
	unknownCount := 0

	for addrStr, node := range nodes {
		// Пропускаем себя
		if node.Address == i.address {
			continue
		}

		// Проверяем, есть ли NetworkAddr (IP:port)
		if node.NetworkAddr != "" && node.NetworkAddr != ":" {
			// Узел имеет IP адрес - помечаем как известный
			knownCount++
			icelogger().Debugw("Node has known IP address",
				"node_address", addrStr,
				"network_addr", node.NetworkAddr,
			)
		} else {
			// Узел не имеет IP адреса - отправляем WHO_IS запрос
			unknownCount++
			icelogger().Infow("Node missing IP address, sending WHO_IS request",
				"node_address", addrStr,
			)
			i.sendWhoIsRequest(addrStr)
		}
	}

	if unknownCount > 0 {
		icelogger().Infow("Verified nodes IP addresses",
			"known_nodes", knownCount,
			"unknown_nodes", unknownCount,
			"total_nodes", len(nodes),
		)
	}
}

// sendWhoIsRequest отправляет WHO_IS запрос для получения IP адреса узла
// Используется только обычными узлами (не bootstrap)
func (i *Ice) sendWhoIsRequest(nodeAddress string) {
	// Формируем запрос: WHO_IS|{node_address}
	message := fmt.Sprintf("%s|%s\n", MsgTypeWhoIs, nodeAddress)

	// Отправляем запрос на bootstrap
	i.mu.Lock()
	conn := i.bootstrapConn
	i.mu.Unlock()

	if conn == nil {
		icelogger().Warnw("Cannot send WHO_IS - bootstrap connection is nil", "node_address", nodeAddress)
		return
	}

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(message)); err != nil {
		icelogger().Warnw("Failed to send WHO_IS request", "node_address", nodeAddress, "err", err)
	} else {
		icelogger().Debugw("Sent WHO_IS request", "node_address", nodeAddress)
	}
}

// processWhoIsResponse обрабатывает WHO_IS ответ и обновляет IP адрес узла
// Используется только обычными узлами (не bootstrap)
func (i *Ice) processWhoIsResponse(data string) {
	// Формат: WHO_IS_RESPONSE|{node_address}|{network_addr}
	parts := splitMessage(data, "|")
	if len(parts) < 3 {
		icelogger().Warnw("Invalid WHO_IS response format", "data", data)
		return
	}

	nodeAddressStr := trimResponse(parts[1])
	networkAddr := trimResponse(parts[2])

	// Парсим адрес узла
	nodeAddr := i.parseAddress(nodeAddressStr)
	if nodeAddr == nil {
		icelogger().Warnw("Failed to parse node address from WHO_IS response", "addr", nodeAddressStr)
		return
	}

	// Обновляем IP адрес узла в консенсусе
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	consensusManager.AddNode(*nodeAddr, networkAddr)

	icelogger().Infow("Updated node network address from WHO_IS response",
		"node_address", nodeAddr.Hex(),
		"network_addr", networkAddr,
	)
}

// isConsensusStatusMessage проверяет, содержит ли сообщение статус консенсуса
func (i *Ice) isConsensusStatusMessage(data string) bool {
	// Формат: CONSENSUS_STATUS|{status}|{voters_addresses}|{nodes_addresses}|{nonce}
	return len(data) > len(MsgTypeConsensusStatus) && strings.HasPrefix(data, MsgTypeConsensusStatus)
}

// processConsensusStatusMessage обрабатывает сообщение со статусом консенсуса
// Используется только обычными узлами (не bootstrap)
func (i *Ice) processConsensusStatusMessage(data string) {
	// Формат: CONSENSUS_STATUS|{status}|{voters_addresses}|{nodes_addresses}|{nonce}
	parts := splitMessage(data, "|")
	if len(parts) < 5 {
		icelogger().Warnw("Invalid CONSENSUS_STATUS message format", "data", data)
		return
	}

	// Парсим статус
	statusStr := trimResponse(parts[1])
	var status int
	if _, err := fmt.Sscanf(statusStr, "%d", &status); err != nil {
		icelogger().Warnw("Failed to parse consensus status", "status_str", statusStr, "err", err)
		return
	}

	// Парсим адреса voters (разделенные запятой)
	votersStr := trimResponse(parts[2])
	var voterAddresses []types.Address
	if votersStr != "" {
		voterAddrs := strings.Split(votersStr, ",")
		for _, addrStr := range voterAddrs {
			addrStr = strings.TrimSpace(addrStr)
			if addrStr != "" {
				addr := types.HexToAddress(addrStr)
				if addr != (types.Address{}) {
					voterAddresses = append(voterAddresses, addr)
				}
			}
		}
	}

	// Парсим адреса nodes (разделенные запятой)
	nodesStr := trimResponse(parts[3])
	var nodeAddresses []types.Address
	if nodesStr != "" {
		nodeAddrs := strings.Split(nodesStr, ",")
		for _, addrStr := range nodeAddrs {
			addrStr = strings.TrimSpace(addrStr)
			if addrStr != "" {
				addr := types.HexToAddress(addrStr)
				if addr != (types.Address{}) {
					nodeAddresses = append(nodeAddresses, addr)
				}
			}
		}
	}

	// Парсим nonce
	nonceStr := trimResponse(parts[4])
	var nonce uint64
	if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
		icelogger().Warnw("Failed to parse nonce", "nonce_str", nonceStr, "err", err)
		return
	}

	// Обновляем статус консенсуса
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	consensusManager.SetStatus(status)

	// Обновляем nonce от bootstrap (bootstrap является источником истины)
	// Всегда обновляем nonce на значение от bootstrap, даже если оно меньше
	// (это может произойти при переподключении или сбросе)
	currentNonce := consensusManager.GetNonce()
	if nonce != currentNonce {
		icelogger().Infow("Updating consensus nonce from CONSENSUS_STATUS",
			"old_nonce", currentNonce,
			"new_nonce", nonce,
		)
		consensusManager.SetNonce(nonce)
		icelogger().Infow("Nonce updated successfully from CONSENSUS_STATUS",
			"new_nonce", nonce,
			"verified_nonce", consensusManager.GetNonce(),
		)
	} else {
		icelogger().Debugw("Nonce already synchronized from CONSENSUS_STATUS", "nonce", nonce)
	}

	// Обновляем список voters в консенсусе
	// consensusManager уже получен выше, используем его
	currentVoters := consensusManager.GetVoters()
	votersMap := make(map[string]bool)
	for _, voter := range currentVoters {
		votersMap[voter.Hex()] = true
	}

	// Добавляем новых voters, которых еще нет
	for _, voterAddr := range voterAddresses {
		if !votersMap[voterAddr.Hex()] {
			consensusManager.AddVoter(voterAddr)
			icelogger().Infow("Added new voter from CONSENSUS_STATUS", "voter_address", voterAddr.Hex())
		}
	}

	// Обновляем список nodes в консенсусе
	currentNodes := consensusManager.GetNodes()
	nodesMap := make(map[string]bool)
	for addrStr := range currentNodes {
		nodesMap[addrStr] = true
	}

	// Добавляем/обновляем nodes из сообщения
	for _, nodeAddr := range nodeAddresses {
		addrStr := nodeAddr.Hex()
		if !nodesMap[addrStr] {
			// Добавляем новый узел (networkAddr будет пустым, обновится позже)
			consensusManager.AddNode(nodeAddr, "")
			icelogger().Infow("Added new node from CONSENSUS_STATUS", "node_address", addrStr)
		} else {
			// Обновляем LastSeen для существующего узла
			consensusManager.UpdateNodeLastSeen(nodeAddr)
			icelogger().Debugw("Updated existing node from CONSENSUS_STATUS", "node_address", addrStr)
		}
	}

	icelogger().Infow("Updated consensus status from bootstrap",
		"status", status,
		"voters_count", len(voterAddresses),
		"nodes_count", len(nodeAddresses),
		"nonce", nonce,
	)
}

// updateConsensusAfterNodes обновляет консенсус после получения списка узлов
// Используется только обычными узлами (не bootstrap)
func (i *Ice) updateConsensusAfterNodes() {
	// Убеждаемся, что текущий узел добавлен в консенсус
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	consensusManager.AddVoter(i.address)
	consensusManager.AddNode(i.address, i.GetNetworkAddr())

	// Получаем информацию о консенсусе
	consensusInfo := consensusManager.GetConsensusInfo()
	icelogger().Infow("Consensus updated after receiving nodes list",
		"address", i.address.Hex(),
		"network_addr", i.GetNetworkAddr(),
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"status", consensusInfo["status"],
		"nonce", consensusInfo["nonce"],
	)
}

// recalculateConsensus пересчитывает консенсус при подключении к bootstrap
// Используется только обычными узлами (не bootstrap)
func (i *Ice) recalculateConsensus() {
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()

	// Добавляем текущий узел в список voters консенсуса
	consensusManager.AddVoter(i.address)

	// Добавляем текущий узел в список nodes
	consensusManager.AddNode(i.address, i.GetNetworkAddr())

	// Обновляем nonce при добавлении узла в консенсус
	oldNonce := consensusManager.GetNonce()
	newNonce := oldNonce + 1
	consensusManager.SetNonce(newNonce)

	// Получаем обновленный nonce
	currentNonce := consensusManager.GetNonce()

	// Не bootstrap узлы не отправляют nonce при пересчете консенсуса
	// Они получают nonce от bootstrap при подключении
	// Bootstrap узел отправляет nonce всем узлам при их подключении в handleReadyRequest

	icelogger().Infow("Recalculated consensus after bootstrap connection",
		"address", i.address.Hex(),
		"network_addr", i.GetNetworkAddr(),
		"nonce", currentNonce,
	)

	// Получаем информацию о консенсусе для логирования
	consensusInfo := consensusManager.GetConsensusInfo()
	icelogger().Infow("Consensus info",
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"status", consensusInfo["status"],
		"nonce", consensusInfo["nonce"],
	)
}
