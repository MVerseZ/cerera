package icenet

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
)

// isReadyRequest проверяет, является ли сообщение запросом на включение
func (i *Ice) isReadyRequest(data string) bool {
	// Формат: READY_REQUEST|{address}|{network_addr}
	return len(data) > 12 && strings.HasPrefix(data, "READY_REQUEST")
}

// isWhoIsRequest проверяет, является ли сообщение WHO_IS запросом
func (i *Ice) isWhoIsRequest(data string) bool {
	// Формат: WHO_IS|{node_address}
	return len(data) > 6 && strings.HasPrefix(data, "WHO_IS")
}

// handleWhoIsRequest обрабатывает WHO_IS запрос (для bootstrap узла)
func (i *Ice) handleWhoIsRequest(conn net.Conn, data string, remoteAddr string) {
	if !i.isBootstrapNode() {
		icelogger().Debugw("Ignoring WHO_IS request - not a bootstrap node")
		return
	}

	icelogger().Infow("Processing WHO_IS request on bootstrap node", "remote_addr", remoteAddr)

	// Формат: WHO_IS|{node_address}
	parts := i.splitMessage(data, "|")
	if len(parts) < 2 {
		icelogger().Warnw("Invalid WHO_IS request format", "data", data)
		return
	}

	nodeAddressStr := i.trimResponse(parts[1])
	icelogger().Infow("Looking up node address in consensus", "requested_address", nodeAddressStr)

	// Ищем узел в консенсусе
	nodes := gigea.GetNodes()
	nodeInfo, exists := nodes[nodeAddressStr]

	if !exists {
		icelogger().Warnw("Node not found in consensus", "node_address", nodeAddressStr, "total_nodes", len(nodes))
		return
	}

	if nodeInfo.NetworkAddr == "" {
		icelogger().Warnw("Node found but has no network address", "node_address", nodeAddressStr)
		return
	}

	// Отправляем ответ с IP адресом узла
	// Формат: WHO_IS_RESPONSE|{node_address}|{network_addr}
	response := fmt.Sprintf("WHO_IS_RESPONSE|%s|%s\n", nodeAddressStr, nodeInfo.NetworkAddr)

	if _, err := conn.Write([]byte(response)); err != nil {
		icelogger().Errorw("Failed to send WHO_IS response", "node_address", nodeAddressStr, "err", err)
	} else {
		icelogger().Infow("Sent WHO_IS response from bootstrap",
			"remote_addr", remoteAddr,
			"node_address", nodeAddressStr,
			"network_addr", nodeInfo.NetworkAddr,
		)
	}
}

// isWhoIsResponse проверяет, является ли сообщение WHO_IS ответом
func (i *Ice) isWhoIsResponse(data string) bool {
	// Формат: WHO_IS_RESPONSE|{node_address}|{network_addr}
	return len(data) > 15 && strings.HasPrefix(data, "WHO_IS_RESPONSE")
}

// handleWhoIsResponse обрабатывает WHO_IS ответ и обновляет IP адрес узла
func (i *Ice) handleWhoIsResponse(data string) {
	// Формат: WHO_IS_RESPONSE|{node_address}|{network_addr}
	parts := i.splitMessage(data, "|")
	if len(parts) < 3 {
		icelogger().Warnw("Invalid WHO_IS response format", "data", data)
		return
	}

	nodeAddressStr := i.trimResponse(parts[1])
	networkAddr := i.trimResponse(parts[2])

	// Парсим адрес узла
	nodeAddr := i.parseAddress(nodeAddressStr)
	if nodeAddr == nil {
		icelogger().Warnw("Failed to parse node address from WHO_IS response", "addr", nodeAddressStr)
		return
	}

	// Обновляем IP адрес узла в консенсусе
	gigea.AddNode(*nodeAddr, networkAddr)

	icelogger().Infow("Updated node network address from WHO_IS response",
		"node_address", nodeAddr.Hex(),
		"network_addr", networkAddr,
	)
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

	// Сохраняем соединение для bootstrap узла ПЕРЕД отправкой ответов
	// чтобы оно было в списке при рассылке статуса
	isBootstrap := i.isBootstrapNode()
	icelogger().Infow("Checking if bootstrap node",
		"is_bootstrap", isBootstrap,
		"network_addr", i.GetNetworkAddr(),
		"bootstrap_addr", i.GetBootstrapAddr(),
	)
	if isBootstrap {
		i.mu.Lock()
		i.connectedNodes[*nodeAddr] = conn
		connectedCount := len(i.connectedNodes)
		i.mu.Unlock()
		icelogger().Infow("Saved connection for node",
			"node_address", nodeAddr.Hex(),
			"total_connected", connectedCount,
		)
	} else {
		icelogger().Warnw("NOT saving connection - not a bootstrap node")
	}

	// Добавляем узел в консенсус
	gigea.AddVoter(*nodeAddr)
	gigea.AddNode(*nodeAddr, networkAddr)

	// Обновляем nonce при добавлении нового узла
	oldNonce := gigea.GetNonce()
	newNonce := oldNonce + 1
	gigea.SetNonce(newNonce)
	icelogger().Infow("Updated nonce after adding node to consensus",
		"old_nonce", oldNonce,
		"new_nonce", newNonce,
		"node_address", nodeAddr.Hex(),
	)

	// Отправляем ответ READY
	readyResponse := "READY\n"
	if _, err := conn.Write([]byte(readyResponse)); err != nil {
		icelogger().Errorw("Error sending READY response", "remote_addr", remoteAddr, "err", err)
		// Удаляем соединение при ошибке
		if i.isBootstrapNode() {
			i.mu.Lock()
			delete(i.connectedNodes, *nodeAddr)
			i.mu.Unlock()
		}
		return
	}

	icelogger().Infow("Sent READY response", "remote_addr", remoteAddr)

	// Отправляем список узлов
	i.sendNodeList(conn, remoteAddr)

	// Отправляем текущий nonce для синхронизации (уже обновленный)
	currentNonce := gigea.GetNonce()
	nonceMessage := fmt.Sprintf("NONCE|%d\n", currentNonce)
	if _, err := conn.Write([]byte(nonceMessage)); err != nil {
		icelogger().Errorw("Error sending nonce", "remote_addr", remoteAddr, "err", err)
		// Удаляем соединение при ошибке
		if i.isBootstrapNode() {
			i.mu.Lock()
			delete(i.connectedNodes, *nodeAddr)
			i.mu.Unlock()
		}
		return
	}

	icelogger().Infow("Sent nonce to new node",
		"remote_addr", remoteAddr,
		"nonce", currentNonce,
	)

	// Рассылаем статус консенсуса и список узлов всем подключенным узлам (включая только что подключившийся)
	if i.isBootstrapNode() {
		icelogger().Infow("Calling broadcastConsensusStatus and broadcastNodeList after new node connection")
		i.broadcastConsensusStatus()
		i.broadcastNodeList()
	} else {
		icelogger().Warnw("Not calling broadcastConsensusStatus - not a bootstrap node",
			"network_addr", i.GetNetworkAddr(),
			"bootstrap_addr", i.GetBootstrapAddr(),
		)
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

	nodeList = append(nodeList, fmt.Sprintf("%s#%s", i.address.Hex(), i.GetNetworkAddr())) // добавляем себя в список узлов (bootstrap узел)

	if len(nodeList) > 0 {
		nodeListStr := ""
		for idx, nodeInfo := range nodeList {
			if idx > 0 {
				nodeListStr += ","
			}
			nodeListStr += nodeInfo
		}

		message := fmt.Sprintf("NODES|%s\n", nodeListStr)
		countMessage := fmt.Sprintf("NODES_COUNT|%d\n", len(nodeList))
		fullMessage := message + countMessage
		if _, err := conn.Write([]byte(fullMessage)); err != nil {
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

// parseAddress парсит адрес из строки
func (i *Ice) parseAddress(addrStr string) *types.Address {
	// Убираем пробелы
	addrStr = i.trimResponse(addrStr)

	// Пробуем распарсить как hex адрес
	addr := types.HexToAddress(addrStr)
	if addr == (types.Address{}) {
		return nil
	}
	return &addr
}

// broadcastConsensusStatus рассылает статус консенсуса всем подключенным узлам
func (i *Ice) broadcastConsensusStatus() {
	if !i.isBootstrapNode() {
		return
	}

	// Получаем информацию о консенсусе
	consensusInfo := gigea.GetConsensusInfo()

	// Получаем адреса voters
	voters := gigea.GetVoters()
	voterAddresses := make([]string, 0, len(voters))
	for _, voter := range voters {
		voterAddresses = append(voterAddresses, voter.Hex())
	}
	votersStr := strings.Join(voterAddresses, ",")

	// Получаем адреса nodes
	nodes := gigea.GetNodes()
	nodeAddresses := make([]string, 0, len(nodes))
	for addr := range nodes {
		nodeAddresses = append(nodeAddresses, addr)
	}
	nodesStr := strings.Join(nodeAddresses, ",")

	// Формируем сообщение со статусом консенсуса
	// Формат: CONSENSUS_STATUS|{status}|{voters_addresses}|{nodes_addresses}|{nonce}
	statusMessage := fmt.Sprintf("CONSENSUS_STATUS|%v|%s|%s|%v\n",
		consensusInfo["status"],
		votersStr,
		nodesStr,
		consensusInfo["nonce"],
	)

	i.mu.Lock()
	// Копируем список соединений для безопасной итерации
	connections := make(map[types.Address]net.Conn)
	for addr, conn := range i.connectedNodes {
		connections[addr] = conn
	}
	totalConnections := len(connections)
	i.mu.Unlock()

	icelogger().Infow("Starting consensus status broadcast",
		"total_connections", totalConnections,
		"status", consensusInfo["status"],
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
	)

	// Рассылаем статус всем подключенным узлам
	sentCount := 0
	failedCount := 0
	for addr, conn := range connections {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write([]byte(statusMessage)); err != nil {
			failedCount++
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
			icelogger().Infow("Successfully sent consensus status to node",
				"node_address", addr.Hex(),
			)
		}
	}

	icelogger().Infow("Consensus status broadcast completed",
		"total_connections", totalConnections,
		"sent_successfully", sentCount,
		"failed", failedCount,
		"status", consensusInfo["status"],
		"voters", consensusInfo["voters"],
		"nodes", consensusInfo["nodes"],
		"nonce", consensusInfo["nonce"],
	)
}

// broadcastNodeList рассылает список узлов всем подключенным узлам
func (i *Ice) broadcastNodeList() {
	if !i.isBootstrapNode() {
		return
	}

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

	nodeList = append(nodeList, fmt.Sprintf("%s#%s", i.address.Hex(), i.GetNetworkAddr())) // добавляем себя в список узлов (bootstrap узел)

	if len(nodeList) == 0 {
		return
	}

	nodeListStr := ""
	for idx, nodeInfo := range nodeList {
		if idx > 0 {
			nodeListStr += ","
		}
		nodeListStr += nodeInfo
	}

	message := fmt.Sprintf("NODES|%s\n", nodeListStr)
	countMessage := fmt.Sprintf("NODES_COUNT|%d\n", len(nodeList))
	fullMessage := message + countMessage

	i.mu.Lock()
	// Копируем список соединений для безопасной итерации
	connections := make(map[types.Address]net.Conn)
	for addr, conn := range i.connectedNodes {
		connections[addr] = conn
	}
	totalConnections := len(connections)
	i.mu.Unlock()

	icelogger().Infow("Starting node list broadcast",
		"total_connections", totalConnections,
		"nodes_count", len(nodeList),
	)

	// Рассылаем список узлов всем подключенным узлам
	sentCount := 0
	failedCount := 0
	for addr, conn := range connections {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write([]byte(fullMessage)); err != nil {
			failedCount++
			icelogger().Warnw("Failed to send node list to node",
				"node_address", addr.Hex(),
				"err", err,
			)
			// Удаляем неработающее соединение
			i.mu.Lock()
			delete(i.connectedNodes, addr)
			i.mu.Unlock()
		} else {
			sentCount++
			icelogger().Infow("Successfully sent node list to node",
				"node_address", addr.Hex(),
			)
		}
	}

	icelogger().Infow("Node list broadcast completed",
		"total_connections", totalConnections,
		"sent_successfully", sentCount,
		"failed", failedCount,
		"nodes_count", len(nodeList),
	)
}

// handleConsensusStatus обрабатывает сообщение CONSENSUS_STATUS
func (i *Ice) handleConsensusStatus(data string) {
	icelogger().Infow("Received CONSENSUS_STATUS message")
	// Используем общую функцию обработки из bootstrap_client.go
	i.processConsensusStatusMessage(data)
}
