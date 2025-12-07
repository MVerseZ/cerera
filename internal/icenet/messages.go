package icenet

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/gigea"
)

// REQ представляет структурированное сообщение с данными для нового узла
// Формат:
// REQ
// A<address>
// NA<network_address>
// N
// NA<node_address1>
// NNA<node_network_address1>
// NA<node_address2>
// NNA<node_network_address2>
// ...
// NONCE<nonce_value>
type REQ struct {
	Address     types.Address // A - адрес bootstrap узла
	NetworkAddr string        // NA - сетевой адрес bootstrap узла
	Nodes       []NodeInfo    // N - список узлов
	Nonce       uint64        // NONCE - текущий nonce
}

// NodeInfo представляет информацию об узле в списке
type NodeInfo struct {
	Address     types.Address // NA - адрес узла
	NetworkAddr string        // NNA - сетевой адрес узла
}

// serializeREQ сериализует структуру REQ в строку для отправки
func (r *REQ) serialize() string {
	var sb strings.Builder

	sb.WriteString("REQ\n")
	sb.WriteString(fmt.Sprintf("A%s\n", r.Address.Hex()))
	sb.WriteString(fmt.Sprintf("NA%s\n", r.NetworkAddr))
	sb.WriteString("N\n")

	for _, node := range r.Nodes {
		sb.WriteString(fmt.Sprintf("NA%s\n", node.Address.Hex()))
		sb.WriteString(fmt.Sprintf("NNA%s\n", node.NetworkAddr))
	}

	sb.WriteString(fmt.Sprintf("NONCE%d\n", r.Nonce))

	return sb.String()
}

// parseREQ парсит строку и создает структуру REQ
func parseREQ(data string) (*REQ, error) {
	req := &REQ{
		Nodes: make([]NodeInfo, 0),
	}

	lines := strings.Split(data, "\n")
	inNodesSection := false
	var currentNode *NodeInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if line == "REQ" {
			continue
		}

		if strings.HasPrefix(line, "A") && !inNodesSection {
			addrStr := strings.TrimPrefix(line, "A")
			addr := types.HexToAddress(addrStr)
			if addr == (types.Address{}) {
				return nil, fmt.Errorf("invalid address: %s", addrStr)
			}
			req.Address = addr
			continue
		}

		if strings.HasPrefix(line, "NA") && !inNodesSection {
			req.NetworkAddr = strings.TrimPrefix(line, "NA")
			continue
		}

		if line == "N" {
			inNodesSection = true
			continue
		}

		if inNodesSection {
			if strings.HasPrefix(line, "NA") {
				addrStr := strings.TrimPrefix(line, "NA")
				addr := types.HexToAddress(addrStr)
				if addr == (types.Address{}) {
					continue // пропускаем невалидные адреса
				}
				currentNode = &NodeInfo{Address: addr}
				continue
			}

			if strings.HasPrefix(line, "NNA") && currentNode != nil {
				currentNode.NetworkAddr = strings.TrimPrefix(line, "NNA")
				req.Nodes = append(req.Nodes, *currentNode)
				currentNode = nil
				continue
			}

			if strings.HasPrefix(line, "NONCE") {
				nonceStr := strings.TrimPrefix(line, "NONCE")
				var nonce uint64
				if _, err := fmt.Sscanf(nonceStr, "%d", &nonce); err != nil {
					return nil, fmt.Errorf("invalid nonce: %s", nonceStr)
				}
				req.Nonce = nonce
				inNodesSection = false
				break
			}
		}
	}

	return req, nil
}

// isREQMessage проверяет, является ли сообщение REQ
func (i *Ice) isREQMessage(data string) bool {
	return strings.HasPrefix(data, "REQ")
}

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

	// Формируем структурированное сообщение REQ
	currentNonce := gigea.GetNonce()

	// Получаем список узлов
	nodes := gigea.GetNodes()
	nodeList := make([]NodeInfo, 0)
	for addr, node := range nodes {
		if node.IsConnected {
			nodeList = append(nodeList, NodeInfo{
				Address:     types.HexToAddress(addr),
				NetworkAddr: node.NetworkAddr,
			})
		}
	}

	// Добавляем bootstrap узел в список
	nodeList = append(nodeList, NodeInfo{
		Address:     i.address,
		NetworkAddr: i.GetNetworkAddr(),
	})

	// Создаем REQ структуру
	req := &REQ{
		Address:     i.address,
		NetworkAddr: i.GetNetworkAddr(),
		Nodes:       nodeList,
		Nonce:       currentNonce,
	}

	// Сериализуем и отправляем REQ
	reqMessage := req.serialize()
	if _, err := conn.Write([]byte(reqMessage)); err != nil {
		icelogger().Errorw("Error sending REQ message", "remote_addr", remoteAddr, "err", err)
		// Удаляем соединение при ошибке
		if i.isBootstrapNode() {
			i.mu.Lock()
			delete(i.connectedNodes, *nodeAddr)
			i.mu.Unlock()
		}
		return
	}

	icelogger().Infow("Sent REQ message to new node",
		"remote_addr", remoteAddr,
		"address", i.address.Hex(),
		"network_addr", i.GetNetworkAddr(),
		"nodes_count", len(nodeList),
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

// isNodeOkMessage проверяет, является ли сообщение подтверждением NODE_OK
func (i *Ice) isNodeOkMessage(data string) bool {
	// Формат: NODE_OK|{count}|{nonce}
	return len(data) > 7 && strings.HasPrefix(data, "NODE_OK")
}

// handleNodeOk обрабатывает сообщение NODE_OK от узла
// Используется только на bootstrap узле
func (i *Ice) handleNodeOk(conn net.Conn, data string, remoteAddr string) {
	if !i.isBootstrapNode() {
		icelogger().Debugw("Ignoring NODE_OK - not a bootstrap node")
		return
	}

	// Формат: NODE_OK|{count}|{nonce}
	parts := i.splitMessage(data, "|")
	if len(parts) < 3 {
		icelogger().Warnw("Invalid NODE_OK message format", "data", data)
		return
	}

	countStr := i.trimResponse(parts[1])
	nonceStr := i.trimResponse(parts[2])

	var nodeCount int
	var nodeNonce uint64

	if _, err := fmt.Sscanf(countStr, "%d", &nodeCount); err != nil {
		icelogger().Warnw("Failed to parse count from NODE_OK", "count_str", countStr, "err", err)
		return
	}

	if _, err := fmt.Sscanf(nonceStr, "%d", &nodeNonce); err != nil {
		icelogger().Warnw("Failed to parse nonce from NODE_OK", "nonce_str", nonceStr, "err", err)
		return
	}

	// Получаем текущий nonce bootstrap узла
	currentNonce := gigea.GetNonce()

	icelogger().Infow("Received NODE_OK from node",
		"remote_addr", remoteAddr,
		"node_count", nodeCount,
		"node_nonce", nodeNonce,
		"bootstrap_nonce", currentNonce,
	)

	// Проверяем синхронизацию nonce
	if nodeNonce != currentNonce {
		icelogger().Warnw("Nonce mismatch in NODE_OK",
			"remote_addr", remoteAddr,
			"node_nonce", nodeNonce,
			"bootstrap_nonce", currentNonce,
		)
	} else {
		icelogger().Infow("Node confirmed successfully with matching nonce",
			"remote_addr", remoteAddr,
			"node_count", nodeCount,
			"nonce", nodeNonce,
		)
	}
}
