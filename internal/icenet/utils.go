package icenet

import (
	"github.com/cerera/internal/cerera/types"
)

// splitMessage разбивает сообщение по разделителю
// Вспомогательная функция для обработки сообщений
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

// trimResponse убирает пробелы и переносы строк из ответа
// Вспомогательная функция для обработки сообщений
func trimResponse(s string) string {
	result := ""
	for _, char := range s {
		if char != ' ' && char != '\n' && char != '\r' && char != '\t' {
			result += string(char)
		}
	}
	return result
}

// addNodeToConsensus добавляет узел в консенсус (общая логика)
// Принимает consensusManager для инжекции зависимости
func addNodeToConsensus(nodeAddr types.Address, networkAddr string, consensusManager ConsensusManager) error {
	// Валидируем адрес
	if err := ValidateAddress(nodeAddr); err != nil {
		return err
	}

	// Валидируем сетевой адрес
	if networkAddr != "" {
		if err := ValidateNetworkAddress(networkAddr); err != nil {
			return err
		}
	}

	// Добавляем узел в консенсус через интерфейс
	consensusManager.AddVoter(nodeAddr)
	consensusManager.AddNode(nodeAddr, networkAddr)

	return nil
}

// addNodesFromList добавляет список узлов в консенсус (общая логика)
// Принимает consensusManager для инжекции зависимости
func addNodesFromList(nodes []NodeInfo, excludeAddr *types.Address, consensusManager ConsensusManager) (int, error) {
	addedCount := 0
	for _, nodeInfo := range nodes {
		// Пропускаем исключенный адрес (обычно свой адрес)
		if excludeAddr != nil && nodeInfo.Address == *excludeAddr {
			continue
		}

		if err := addNodeToConsensus(nodeInfo.Address, nodeInfo.NetworkAddr, consensusManager); err != nil {
			icelogger().Warnw("Failed to add node to consensus",
				"address", nodeInfo.Address.Hex(),
				"network_addr", nodeInfo.NetworkAddr,
				"err", err)
			continue
		}

		addedCount++
		icelogger().Debugw("Added node to consensus",
			"address", nodeInfo.Address.Hex(),
			"network_addr", nodeInfo.NetworkAddr,
		)
	}

	return addedCount, nil
}

