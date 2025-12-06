package icenet

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
