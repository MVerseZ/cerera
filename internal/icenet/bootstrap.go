package icenet

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
)

// connectToBootstrap подключается к bootstrap узлу и поддерживает соединение постоянно
func (i *Ice) connectToBootstrap() {
	bootstrapAddr := i.GetBootstrapAddr()
	retryDelay := 5 * time.Second

	for {
		i.mu.Lock()
		started := i.started
		i.mu.Unlock()

		if !started {
			return
		}

		icelogger().Infow("Connecting to bootstrap node", "bootstrap_addr", bootstrapAddr)

		conn, err := net.DialTimeout("tcp", bootstrapAddr, 10*time.Second)
		if err != nil {
			icelogger().Warnw("Failed to connect to bootstrap",
				"bootstrap_addr", bootstrapAddr,
				"err", err,
			)
			time.Sleep(retryDelay)
			continue
		}

		icelogger().Infow("Successfully connected to bootstrap", "bootstrap_addr", bootstrapAddr)

		// Сохраняем соединение
		i.mu.Lock()
		i.bootstrapConn = conn
		i.mu.Unlock()

		// Добавляем текущий узел в консенсус
		i.recalculateConsensus()

		// Отправляем запрос на включение с информацией о себе
		readyRequest := fmt.Sprintf("READY_REQUEST|%s|%s\n", i.address.Hex(), i.GetNetworkAddr())
		if _, err := conn.Write([]byte(readyRequest)); err != nil {
			icelogger().Errorw("Error sending ready request to bootstrap", "err", err)
			conn.Close()
			i.mu.Lock()
			i.bootstrapConn = nil
			i.mu.Unlock()
			time.Sleep(retryDelay)
			continue
		}

		// Запускаем горутину для чтения ответов от bootstrap
		go i.readFromBootstrap(conn)

		// Отправляем keep-alive пакеты периодически
		go i.sendKeepAlive(conn)

		// Ждем разрыва соединения
		// Проверяем соединение периодически
		ticker := time.NewTicker(10 * time.Second)
		connectionAlive := true
		for connectionAlive {
			select {
			case <-ticker.C:
				// Проверяем, что соединение еще живо
				conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
				if _, err := conn.Write([]byte("PING\n")); err != nil {
					icelogger().Warnw("Bootstrap connection lost, reconnecting", "err", err)
					conn.Close()
					i.mu.Lock()
					i.bootstrapConn = nil
					i.mu.Unlock()
					i.resetBootstrapReady()
					ticker.Stop()
					connectionAlive = false
				}
			case <-i.ctx.Done():
				ticker.Stop()
				conn.Close()
				i.mu.Lock()
				i.bootstrapConn = nil
				i.mu.Unlock()
				return
			}
		}

		// Если соединение разорвано, ждем перед переподключением
		time.Sleep(retryDelay)
	}
}

// readFromBootstrap читает данные от bootstrap соединения
func (i *Ice) readFromBootstrap(conn net.Conn) {
	buffer := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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
			data := string(buffer[:n])
			icelogger().Infow("Received data from bootstrap",
				"size", n,
				"data", data,
			)

			// Проверяем, является ли это подтверждающим ответом о готовности
			if i.isReadyResponse(data) {
				i.setBootstrapReady()
			}

			// Проверяем, является ли это командой начала консенсуса
			if i.isConsensusStartCommand(data) {
				i.startConsensus()
			}

			// Проверяем, содержит ли сообщение список узлов для консенсуса
			if i.isNodeListMessage(data) {
				i.processNodeListMessage(data)
			}

			// Проверяем, содержит ли сообщение nonce для синхронизации
			if i.isNonceMessage(data) {
				i.processNonceMessage(data)
			}

			// Проверяем, содержит ли сообщение broadcast nonce
			if i.isBroadcastNonceMessage(data) {
				i.processBroadcastNonceMessage(data)
			}
		}
	}
}

// isNodeListMessage проверяет, содержит ли сообщение список узлов
func (i *Ice) isNodeListMessage(data string) bool {
	// Формат: NODES|address1#network_addr1,address2#network_addr2,...
	return len(data) > 5 && data[:5] == "NODES"
}

// processNodeListMessage обрабатывает сообщение со списком узлов и добавляет их в консенсус
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

			// Парсим адрес
			nodeAddr := i.parseAddress(nodeAddrStr)
			if nodeAddr == nil {
				icelogger().Warnw("Failed to parse node address", "addr", nodeAddrStr)
				continue
			}

			// Добавляем узел в консенсус
			gigea.AddVoter(*nodeAddr)
			gigea.AddNode(*nodeAddr, networkAddr)
			addedCount++

			icelogger().Debugw("Added node to consensus",
				"address", nodeAddr.Hex(),
				"network_addr", networkAddr,
			)
		}
	}

	if addedCount > 0 {
		icelogger().Infow("Processed node list from bootstrap",
			"nodes_added", addedCount,
		)

		// Получаем обновленную информацию о консенсусе
		consensusInfo := gigea.GetConsensusInfo()
		icelogger().Infow("Updated consensus info",
			"voters", consensusInfo["voters"],
			"nodes", consensusInfo["nodes"],
		)

		// Не bootstrap узлы не отправляют nonce - они получают его от bootstrap
	}
}

// splitMessage разбивает сообщение по разделителю
func splitMessage(msg, delimiter string) []string {
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

// parseAddress парсит адрес из строки
func (i *Ice) parseAddress(addrStr string) *types.Address {
	// Убираем пробелы
	addrStr = trimResponse(addrStr)

	// Пробуем распарсить как hex адрес
	addr := types.HexToAddress(addrStr)
	if addr == (types.Address{}) {
		return nil
	}
	return &addr
}

// isNonceMessage проверяет, содержит ли сообщение nonce
func (i *Ice) isNonceMessage(data string) bool {
	// Формат: NONCE|{nonce_value}
	return len(data) > 5 && data[:5] == "NONCE"
}

// processNonceMessage обрабатывает сообщение с nonce и обновляет консенсус
func (i *Ice) processNonceMessage(data string) {
	// Формат: NONCE|{nonce_value}
	parts := splitMessage(data, "|")
	if len(parts) < 2 {
		icelogger().Warnw("Invalid nonce message format", "data", data)
		return
	}

	nonceStr := trimResponse(parts[1])
	var nonce uint64
	if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
		icelogger().Warnw("Failed to parse nonce", "nonce_str", nonceStr, "err", err)
		return
	}

	// Обновляем nonce в консенсусе
	currentNonce := gigea.GetNonce()
	if nonce != currentNonce {
		icelogger().Infow("Updating consensus nonce",
			"old_nonce", currentNonce,
			"new_nonce", nonce,
		)
		gigea.SetNonce(nonce)
	} else {
		icelogger().Debugw("Nonce already synchronized", "nonce", nonce)
	}
}

// isBroadcastNonceMessage проверяет, содержит ли сообщение broadcast nonce
func (i *Ice) isBroadcastNonceMessage(data string) bool {
	// Формат: BROADCAST_NONCE|{nonce_value}|{node_list}
	return len(data) > 15 && data[:15] == "BROADCAST_NONCE"
}

// processBroadcastNonceMessage обрабатывает сообщение с broadcast nonce
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
	currentNonce := gigea.GetNonce()
	if nonce != currentNonce {
		icelogger().Infow("Updating consensus nonce from broadcast",
			"old_nonce", currentNonce,
			"new_nonce", nonce,
		)
		gigea.SetNonce(nonce)
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

// isReadyResponse проверяет, является ли ответ подтверждением готовности
func (i *Ice) isReadyResponse(data string) bool {
	// Проверяем различные варианты ответов о готовности
	responses := []string{"READY", "READY_OK", "OK", "ACCEPTED"}
	data = trimResponse(data)
	for _, resp := range responses {
		if data == resp {
			return true
		}
	}
	return false
}

// trimResponse убирает пробелы и переносы строк из ответа
func trimResponse(s string) string {
	result := ""
	for _, char := range s {
		if char != ' ' && char != '\n' && char != '\r' && char != '\t' {
			result += string(char)
		}
	}
	return result
}

// isConsensusStartCommand проверяет, является ли сообщение командой начала консенсуса
func (i *Ice) isConsensusStartCommand(data string) bool {
	// Проверяем различные варианты команд начала консенсуса
	commands := []string{"START_CONSENSUS", "CONSENSUS_START", "BEGIN_CONSENSUS", "CONSENSUS_BEGIN"}
	data = trimResponse(data)
	for _, cmd := range commands {
		if data == cmd {
			return true
		}
	}
	return false
}

// recalculateConsensus пересчитывает консенсус при подключении к bootstrap
func (i *Ice) recalculateConsensus() {
	// Добавляем текущий узел в список voters консенсуса
	gigea.AddVoter(i.address)

	// Добавляем текущий узел в список nodes
	gigea.AddNode(i.address, i.GetNetworkAddr())

	// Получаем текущий nonce
	currentNonce := gigea.GetNonce()

	// Не bootstrap узлы не отправляют nonce при пересчете консенсуса
	// Они получают nonce от bootstrap при подключении
	// Bootstrap узел отправляет nonce всем узлам при их подключении в handleReadyRequest

	icelogger().Infow("Recalculated consensus after bootstrap connection",
		"address", i.address.Hex(),
		"network_addr", i.GetNetworkAddr(),
		"nonce", currentNonce,
	)

	// Получаем информацию о консенсусе для логирования
	consensusInfo := gigea.GetConsensusInfo()
	icelogger().Infow("Consensus info",
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"status", consensusInfo["status"],
		"nonce", consensusInfo["nonce"],
	)
}

// sendNonceToBootstrap отправляет nonce на bootstrap для синхронизации
func (i *Ice) sendNonceToBootstrap(nonce uint64) {
	i.mu.Lock()
	conn := i.bootstrapConn
	i.mu.Unlock()

	if conn == nil {
		return
	}

	// Формируем сообщение: NONCE|{nonce_value}
	message := fmt.Sprintf("NONCE|%d\n", nonce)
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(message)); err != nil {
		icelogger().Warnw("Failed to send nonce to bootstrap", "err", err)
	} else {
		icelogger().Debugw("Sent nonce to bootstrap", "nonce", nonce)
	}
}

// broadcastNonceToAllNodes отправляет nonce всем узлам через bootstrap
func (i *Ice) broadcastNonceToAllNodes(nonce uint64) {
	// Получаем список всех узлов из консенсуса
	nodes := gigea.GetNodes()
	if len(nodes) == 0 {
		return
	}

	// Формируем сообщение со списком узлов и nonce
	// Формат: BROADCAST_NONCE|{nonce_value}|{node1_address#node1_network_addr,node2_address#node2_network_addr,...}
	var nodeList []string
	for addr, node := range nodes {
		if node.IsConnected {
			nodeList = append(nodeList, fmt.Sprintf("%s#%s", addr, node.NetworkAddr))
		}
	}

	if len(nodeList) > 0 {
		// Объединяем все узлы через запятую
		nodeListStr := ""
		for idx, nodeInfo := range nodeList {
			if idx > 0 {
				nodeListStr += ","
			}
			nodeListStr += nodeInfo
		}

		message := fmt.Sprintf("BROADCAST_NONCE|%d|%s\n", nonce, nodeListStr)

		i.mu.Lock()
		conn := i.bootstrapConn
		i.mu.Unlock()

		if conn != nil {
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := conn.Write([]byte(message)); err != nil {
				icelogger().Warnw("Failed to broadcast nonce to nodes", "err", err)
			} else {
				icelogger().Infow("Broadcasted nonce to all nodes",
					"nonce", nonce,
					"nodes_count", len(nodeList),
				)
			}
		}
	}
}

// sendKeepAlive отправляет keep-alive пакеты для поддержания соединения
func (i *Ice) sendKeepAlive(conn net.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			i.mu.Lock()
			started := i.started
			currentConn := i.bootstrapConn
			i.mu.Unlock()

			if !started || currentConn != conn {
				return
			}

			if _, err := conn.Write([]byte("KEEPALIVE\n")); err != nil {
				icelogger().Warnw("Failed to send keep-alive", "err", err)
				return
			}
		case <-i.ctx.Done():
			return
		}
	}
}
