package icenet

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/types"
	"github.com/cerera/internal/icenet/connection"
	"github.com/cerera/internal/icenet/protocol"
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
	Address     types.Address // A - адрес узла
	NetworkAddr string        // NA - сетевой адрес узла
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
	return strings.HasPrefix(data, MsgTypeREQ)
}

// isBlockMessage проверяет, является ли сообщение блоком
func (i *Ice) isBlockMessage(data string) bool {
	// Формат: BLOCK|<json_data>
	return len(data) > len(MsgTypeBlock) && strings.HasPrefix(data, MsgTypeBlock)
}

// parseBlockMessage парсит JSON блок из сообщения
// Формат сообщения: BLOCK|<json_data>
func (i *Ice) parseBlockMessage(data string) (*block.Block, error) {
	// Извлекаем JSON часть после "BLOCK|"
	parts := i.splitMessage(data, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid block message format: missing JSON data")
	}

	// JSON данные находятся во второй части
	jsonData := parts[1]
	// Убираем возможные пробелы и переносы строк
	jsonData = strings.TrimSpace(jsonData)

	// Парсим JSON в структуру Block
	var b block.Block
	if err := json.Unmarshal([]byte(jsonData), &b); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block JSON: %w", err)
	}

	// Проверяем базовую валидность блока
	if b.Head == nil {
		return nil, fmt.Errorf("block header is nil")
	}

	return &b, nil
}

// isReadyRequest проверяет, является ли сообщение запросом на включение
func (i *Ice) isReadyRequest(data string) bool {
	// Формат: READY_REQUEST|{address}|{network_addr}
	return len(data) > len(MsgTypeReadyRequest) && strings.HasPrefix(data, MsgTypeReadyRequest)
}

// isWhoIsRequest проверяет, является ли сообщение WHO_IS запросом
func (i *Ice) isWhoIsRequest(data string) bool {
	// Формат: WHO_IS|{node_address}
	return len(data) > len(MsgTypeWhoIs) && strings.HasPrefix(data, MsgTypeWhoIs)
}

// handleWhoIsRequest обрабатывает WHO_IS запрос
func (i *Ice) handleWhoIsRequest(conn net.Conn, data string, remoteAddr string) {
	icelogger().Infow("Processing WHO_IS request", "remote_addr", remoteAddr)

	// Формат: WHO_IS|{node_address}
	parts := i.splitMessage(data, "|")
	if len(parts) < 2 {
		icelogger().Warnw("Invalid WHO_IS request format", "data", data)
		return
	}

	nodeAddressStr := i.trimResponse(parts[1])
	icelogger().Infow("Looking up node address in consensus", "requested_address", nodeAddressStr)

	// Ищем узел в консенсусе
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	nodes := consensusManager.GetNodes()
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
	response := fmt.Sprintf("%s|%s|%s\n", MsgTypeWhoIsResponse, nodeAddressStr, nodeInfo.NetworkAddr)

	if _, err := conn.Write([]byte(response)); err != nil {
		icelogger().Errorw("Failed to send WHO_IS response", "node_address", nodeAddressStr, "err", err)
	} else {
		icelogger().Infow("Sent WHO_IS response",
			"remote_addr", remoteAddr,
			"node_address", nodeAddressStr,
			"network_addr", nodeInfo.NetworkAddr,
		)
	}
}

// isWhoIsResponse проверяет, является ли сообщение WHO_IS ответом
func (i *Ice) isWhoIsResponse(data string) bool {
	// Формат: WHO_IS_RESPONSE|{node_address}|{network_addr}
	return len(data) > len(MsgTypeWhoIsResponse) && strings.HasPrefix(data, MsgTypeWhoIsResponse)
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
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	consensusManager.AddNode(*nodeAddr, networkAddr)

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

	// Валидируем сетевой адрес
	if err := ValidateNetworkAddress(networkAddr); err != nil {
		icelogger().Warnw("Invalid network address in READY_REQUEST", "network_addr", networkAddr, "err", err)
		return
	}

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

	// Добавляем узел в PeerStore для P2P сети
	if i.peerStore != nil {
		i.peerStore.AddOrUpdate(*nodeAddr, networkAddr)
		icelogger().Infow("Added node to peer store",
			"node_address", nodeAddr.Hex(),
			"network_addr", networkAddr,
		)
	}

	// Добавляем узел в консенсус (используем общую функцию)
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	if err := addNodeToConsensus(*nodeAddr, networkAddr, consensusManager); err != nil {
		icelogger().Warnw("Failed to add node to consensus in READY_REQUEST",
			"node_address", nodeAddr.Hex(),
			"network_addr", networkAddr,
			"err", err)
		return
	}

	// Обновляем nonce при добавлении нового узла
	// consensusManager уже получен выше, используем его
	oldNonce := consensusManager.GetNonce()
	newNonce := oldNonce + 1
	consensusManager.SetNonce(newNonce)
	icelogger().Infow("Updated nonce after adding node to consensus",
		"old_nonce", oldNonce,
		"new_nonce", newNonce,
		"node_address", nodeAddr.Hex(),
	)

	// Формируем структурированное сообщение REQ
	// consensusManager уже получен выше, используем его
	currentNonce := consensusManager.GetNonce()

	// Получаем список узлов
	nodes := consensusManager.GetNodes()
	nodeList := make([]NodeInfo, 0, len(nodes)+1)
	for addr, node := range nodes {
		if node.IsConnected {
			nodeList = append(nodeList, NodeInfo{
				Address:     types.HexToAddress(addr),
				NetworkAddr: node.NetworkAddr,
			})
		}
	}

	// Добавляем текущий узел в список
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
		return
	}

	icelogger().Infow("Sent REQ message to new node",
		"remote_addr", remoteAddr,
		"address", i.address.Hex(),
		"network_addr", i.GetNetworkAddr(),
		"nodes_count", len(nodeList),
		"nonce", currentNonce,
	)

	// Рассылаем статус консенсуса и список узлов через gossip
	if i.meshNetwork != nil {
		// Создаем NODES сообщение для распространения информации о новом узле
		nodeInfos := make([]protocol.NodeInfo, 0, len(nodeList)+1)
		for _, node := range nodeList {
			nodeInfos = append(nodeInfos, protocol.NodeInfo{
				Address:     node.Address,
				NetworkAddr: node.NetworkAddr,
			})
		}
		// Добавляем новый узел
		nodeInfos = append(nodeInfos, protocol.NodeInfo{
			Address:     *nodeAddr,
			NetworkAddr: networkAddr,
		})
		
		nodesMsg := &protocol.NodesMessage{Nodes: nodeInfos}
		if err := i.meshNetwork.BroadcastMessage(nodesMsg); err != nil {
			icelogger().Warnw("Failed to broadcast nodes via gossip",
				"node_address", nodeAddr.Hex(),
				"err", err)
		} else {
			icelogger().Infow("Broadcasted nodes via gossip",
				"node_address", nodeAddr.Hex(),
			)
		}
	} else {
		icelogger().Infow("Node added to consensus, mesh network not available for gossip",
			"node_address", nodeAddr.Hex(),
		)
	}
}

// splitMessage разбивает сообщение по разделителю (обертка для совместимости)
func (i *Ice) splitMessage(msg, delimiter string) []string {
	return splitMessage(msg, delimiter)
}

// trimResponse убирает пробелы и переносы строк из ответа (обертка для совместимости)
func (i *Ice) trimResponse(s string) string {
	return trimResponse(s)
}

// parseAddress парсит адрес из строки с валидацией
func (i *Ice) parseAddress(addrStr string) *types.Address {
	// Убираем пробелы
	addrStr = i.trimResponse(addrStr)

	// Валидируем и парсим адрес
	addr, err := ValidateHexAddress(addrStr)
	if err != nil {
		icelogger().Warnw("Failed to validate address", "addr", addrStr, "err", err)
		return nil
	}
	return addr
}

// broadcastConsensusStatus рассылает статус консенсуса всем подключенным узлам
// TODO: Переделать на gossip-based рассылку
func (i *Ice) broadcastConsensusStatus() {

	// Получаем информацию о консенсусе
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	consensusInfo := consensusManager.GetConsensusInfo()

	// Получаем адреса voters и nodes
	voters := consensusManager.GetVoters()
	nodes := consensusManager.GetNodes()
	
	// Используем mesh network для рассылки через gossip
	if i.meshNetwork != nil {
		// Получаем адреса узлов для сообщения
		nodeAddressList := make([]types.Address, 0, len(nodes))
		for addr := range nodes {
			nodeAddressList = append(nodeAddressList, types.HexToAddress(addr))
		}
		
		// Создаем сообщение для отправки
		statusMsg := &protocol.ConsensusStatusMessage{
			Status: int(consensusInfo["status"].(int)),
			Voters: voters,
			Nodes:  nodeAddressList,
			Nonce:  consensusInfo["nonce"].(uint64),
		}
		
		if err := i.meshNetwork.BroadcastMessage(statusMsg); err != nil {
			icelogger().Warnw("Failed to broadcast consensus status via gossip", "err", err)
		} else {
			icelogger().Infow("Broadcasted consensus status via gossip",
				"status", consensusInfo["status"],
				"voters", len(voters),
				"nodes", len(nodes),
			)
		}
		return
	}

	// Fallback на ConnectionManager если mesh network не доступен
	allConnections := i.connManager.GetAllConnections()
	totalConnections := len(allConnections)

	icelogger().Infow("Starting consensus status broadcast (fallback)",
		"total_connections", totalConnections,
		"status", consensusInfo["status"],
		"voters", len(voters),
		"nodes", len(nodes),
	)

	// Рассылаем статус всем подключенным узлам
	sentCount := 0
	failedCount := 0
	handler := i.connManager.GetHandler()
	
	// Получаем адреса узлов для сообщения
	nodeAddressList := make([]types.Address, 0, len(nodes))
	for addr := range nodes {
		nodeAddressList = append(nodeAddressList, types.HexToAddress(addr))
	}
	
	// Создаем сообщение для отправки
	statusMsg := &protocol.ConsensusStatusMessage{
		Status: int(consensusInfo["status"].(int)),
		Voters: voters,
		Nodes:  nodeAddressList,
		Nonce:  consensusInfo["nonce"].(uint64),
	}
	
	for _, conn := range allConnections {
		if conn.State != connection.StateConnected {
			continue
		}
		
		if err := handler.WriteMessage(conn, statusMsg); err != nil {
			failedCount++
			icelogger().Warnw("Failed to send consensus status to node",
				"node_address", conn.RemoteAddr.Hex(),
				"err", err,
			)
		} else {
			sentCount++
			icelogger().Debugw("Successfully sent consensus status to node",
				"node_address", conn.RemoteAddr.Hex(),
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

// broadcastNodeList рассылает список узлов всем подключенным узлам через gossip
func (i *Ice) broadcastNodeList() {

	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	nodes := consensusManager.GetNodes()
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

	nodeList = append(nodeList, fmt.Sprintf("%s#%s", i.address.Hex(), i.GetNetworkAddr())) // добавляем себя в список узлов

	if len(nodeList) == 0 {
		return
	}

	// Используем mesh network для рассылки через gossip
	if i.meshNetwork != nil {
		// Создаем NodeInfo список для протокола
		nodeInfos := make([]protocol.NodeInfo, 0, len(nodeList))
		for _, nodeStr := range nodeList {
			parts := strings.Split(nodeStr, "#")
			if len(parts) == 2 {
				addr := types.HexToAddress(parts[0])
				nodeInfos = append(nodeInfos, protocol.NodeInfo{
					Address:     addr,
					NetworkAddr: parts[1],
				})
			}
		}
		
		nodesMsg := &protocol.NodesMessage{Nodes: nodeInfos}
		countMsg := &protocol.NodesCountMessage{Count: len(nodeList)}
		
		if err := i.meshNetwork.BroadcastMessage(nodesMsg); err != nil {
			icelogger().Warnw("Failed to broadcast nodes via gossip", "err", err)
		} else {
			icelogger().Infow("Broadcasted nodes via gossip",
				"nodes_count", len(nodeList),
			)
		}
		
		// Также отправляем count сообщение
		if err := i.meshNetwork.BroadcastMessage(countMsg); err != nil {
			icelogger().Warnw("Failed to broadcast nodes count via gossip", "err", err)
		}
		return
	}

	// Fallback на ConnectionManager если mesh network не доступен
	allConnections := i.connManager.GetAllConnections()
	totalConnections := len(allConnections)

	icelogger().Infow("Starting node list broadcast (fallback)",
		"total_connections", totalConnections,
		"nodes_count", len(nodeList),
	)

	// Рассылаем список узлов всем подключенным узлам
	sentCount := 0
	failedCount := 0
	handler := i.connManager.GetHandler()
	
	// Создаем NodeInfo список для протокола
	nodeInfos := make([]protocol.NodeInfo, 0, len(nodeList))
	for _, nodeStr := range nodeList {
		parts := strings.Split(nodeStr, "#")
		if len(parts) == 2 {
			addr := types.HexToAddress(parts[0])
			nodeInfos = append(nodeInfos, protocol.NodeInfo{
				Address:     addr,
				NetworkAddr: parts[1],
			})
		}
	}
	
	nodesMsg := &protocol.NodesMessage{Nodes: nodeInfos}
	countMsg := &protocol.NodesCountMessage{Count: len(nodeList)}
	
	for _, conn := range allConnections {
		if conn.State != connection.StateConnected {
			continue
		}
		
		if err := handler.WriteMessage(conn, nodesMsg); err != nil {
			failedCount++
			icelogger().Warnw("Failed to send node list to node",
				"node_address", conn.RemoteAddr.Hex(),
				"err", err,
			)
		} else {
			sentCount++
			// Отправляем также count сообщение
			handler.WriteMessage(conn, countMsg)
			icelogger().Infow("Successfully sent node list to node",
				"node_address", conn.RemoteAddr.Hex(),
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
	return len(data) > len(MsgTypeNodeOk) && strings.HasPrefix(data, MsgTypeNodeOk)
}

// handleNodeOk обрабатывает сообщение NODE_OK от узла
// TODO: Переделать для P2P сети с использованием gossip
func (i *Ice) handleNodeOk(conn net.Conn, data string, remoteAddr string) {

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

	// Получаем текущий nonce узла
	i.mu.RLock()
	consensusManager := i.consensusManager
	i.mu.RUnlock()
	currentNonce := consensusManager.GetNonce()

	icelogger().Debugw("Received NODE_OK from node",
		"remote_addr", remoteAddr,
		"node_count", nodeCount,
		"node_nonce", nodeNonce,
		"current_nonce", currentNonce,
	)

	// Проверяем синхронизацию nonce
	if nodeNonce != currentNonce {
		icelogger().Warnw("Nonce mismatch in NODE_OK",
			"remote_addr", remoteAddr,
			"node_nonce", nodeNonce,
			"current_nonce", currentNonce,
		)
		return
	}
	
	icelogger().Infow("Received NODE_OK confirmation",
		"remote_addr", remoteAddr,
		"node_count", nodeCount,
		"nonce", nodeNonce,
	)
	
	// В P2P сети NODE_OK может использоваться для обновления информации о пире
	// Обновляем last seen в peer store если возможно
	// (адрес узла нужно получить из соединения)
}
