package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type nodeView string

const (
	viewOverview nodeView = "overview"
	viewBlock    nodeView = "block"
	viewPool     nodeView = "pool"
)

func runTUI(args []string) {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	endpointDefault := os.Getenv("CERERA_RPC_URL")
	if endpointDefault == "" {
		endpointDefault = "http://127.0.0.1:8080/"
	}
	endpoint := fs.String("endpoint", endpointDefault, "Node JSON-RPC endpoint")
	nodeBin := fs.String("node-bin", defaultNodeBinary(), "Path to the cerera executable used by s/x")
	nodeKey := fs.String("node-key", "ddddd.nodekey.pem", "Node identity key used for transfers and local startup")
	dataDirDefault := os.Getenv("CERERA_DATA_DIR")
	if dataDirDefault == "" {
		dataDirDefault = "."
	}
	dataDir := fs.String("data-dir", dataDirDefault, "Node data directory used when starting a local node")
	p2pPort := fs.String("port", "31000", "P2P port used when starting a local node")
	httpPort := fs.Int("http", 0, "HTTP RPC port used when starting a local node (defaults to --endpoint port)")
	refreshInterval := fs.Duration("refresh", 3*time.Second, "How often to refresh node data")
	mining := fs.Bool("miner", true, "Enable mining when starting a local node")
	if err := fs.Parse(args); err != nil {
		return
	}
	if *httpPort < 0 || *httpPort > 65535 || (hasFlag(args, "-http", "--http") && *httpPort == 0) {
		fmt.Fprintln(os.Stderr, "tui: --http must be between 1 and 65535")
		return
	}
	if *httpPort > 0 && !hasFlag(args, "-endpoint", "--endpoint") {
		*endpoint = endpointWithPort(*endpoint, *httpPort)
	}
	if strings.TrimSpace(*endpoint) == "" {
		fmt.Fprintln(os.Stderr, "tui: --endpoint must not be empty")
		return
	}
	if *refreshInterval <= 0 {
		fmt.Fprintln(os.Stderr, "tui: --refresh must be greater than zero")
		return
	}
	restoreTerminal := setupTerminal()
	defer restoreTerminal()

	view := viewOverview
	var managed *managedNode
	message := ""
	confirmStop := false
	ticker := time.NewTicker(*refreshInterval)
	defer ticker.Stop()
	type keyResult struct {
		key rune
		err error
	}
	keys := make(chan keyResult, 1)
	keyPending := false
	for {
		if !keyPending {
			keyPending = true
			go func() {
				key, err := readKey()
				keys <- keyResult{key: key, err: err}
			}()
		}
		drawNodeScreen(*endpoint, view, managed, *nodeBin, message, confirmStop)
		if terminalNeedsEnter {
			fmt.Print("\n[1] Обзор  [2] Блок  [3] Mempool  [t] Перевод  [s] Старт  [x] Стоп  [r] Обновить  [q] Выход (Enter)\n")
		} else {
			fmt.Print("\n[1] Обзор  [2] Блок  [3] Mempool  [t] Перевод  [s] Старт  [x] Стоп  [r] Обновить  [q] Выход\n")
		}
		var key rune
		select {
		case <-ticker.C:
			continue
		case result := <-keys:
			keyPending = false
			key = result.key
			if result.err != nil {
				if result.err == errUnsupportedKey {
					continue
				}
				return
			}
		}
		message = ""
		if confirmStop {
			switch strings.ToLower(string(key)) {
			case "y":
				if managed == nil {
					message = "Нет процесса, запущенного этой консолью"
				} else if err := stopManagedNode(managed); err != nil {
					message = "Не удалось остановить ноду: " + err.Error()
				} else {
					managed = nil
					message = "Локальная нода остановлена"
				}
				confirmStop = false
			case "n", "q", "x":
				confirmStop = false
				message = "Остановка отменена"
			default:
				continue
			}
			continue
		}
		switch strings.ToLower(string(key)) {
		case "q":
			return
		case "1", "o":
			view = viewOverview
		case "2", "b":
			view = viewBlock
		case "3", "p":
			view = viewPool
		case "t":
			message = sendFromTUI(*endpoint, *nodeKey, *dataDir)
		case "s":
			if !isLocalEndpoint(*endpoint) {
				message = "Запуск доступен только для localhost; удалённую ноду запускает её владелец"
			} else if managed != nil {
				managed.refresh()
				if managed.running() {
					message = "Эта консоль уже запустила ноду"
				} else {
					if managed.exitErr != nil {
						message = "Предыдущий процесс завершился: " + managed.exitErr.Error()
					}
					managed = nil
				}
			}
			if message == "" && managed == nil {
				if _, _, err := queryNodeHealth(*endpoint); err == nil {
					message = "На этом RPC endpoint уже отвечает нода; второй процесс не запущен"
				} else if *nodeBin == "" {
					message = "Не найден cerera; задайте путь через --node-bin"
				} else {
					port := *httpPort
					if port == 0 {
						port = endpointPort(*endpoint)
					}
					managed, err = startManagedNode(*nodeBin, *dataDir, *p2pPort, port, endpointUsesTLS(*endpoint), *mining, *nodeKey)
					if err != nil {
						message = "Не удалось запустить ноду: " + err.Error()
					} else {
						message = "Запуск ноды инициирован; статус появится после готовности RPC"
					}
				}
			}
		case "x":
			if managed == nil {
				message = "Эта консоль может остановить только запущенный ею процесс"
			} else {
				confirmStop = true
			}
		case "", "r":
		default:
			continue
		}
	}
}

func hasFlag(args []string, names ...string) bool {
	for _, arg := range args {
		for _, name := range names {
			if arg == name || strings.HasPrefix(arg, name+"=") {
				return true
			}
		}
	}
	return false
}

func drawNodeScreen(endpoint string, view nodeView, managed *managedNode, nodeBin, message string, confirmStop bool) {
	// ANSI clear keeps the interface usable in common Windows and Unix terminals.
	fmt.Print("\033[2J\033[H")
	fmt.Printf("CERERA CTL   %s   %s\n", time.Now().Format("15:04:05"), endpoint)
	fmt.Println(strings.Repeat("─", 72))
	if managed != nil {
		managed.refresh()
	}
	if managed != nil && managed.running() {
		fmt.Printf("Процесс: запущен этой консолью (PID %d)\nЛог: %s\n", managed.cmd.Process.Pid, managed.logPath)
	} else if managed != nil {
		if managed.exitErr != nil {
			fmt.Printf("Процесс завершился с ошибкой: %v\nЛог: %s\n", managed.exitErr, managed.logPath)
		} else {
			fmt.Printf("Процесс остановлен\nЛог: %s\n", managed.logPath)
		}
	} else {
		fmt.Printf("Процесс: %s\n", nodeProcessSummary(endpoint, nodeBin))
	}

	switch view {
	case viewBlock:
		renderLatestBlock(endpoint)
	case viewPool:
		renderJSONResult(endpoint, "cerera.pool.getInfo", "Mempool")
	default:
		renderOverview(endpoint)
	}
	if message != "" {
		fmt.Printf("\n%s\n", message)
	}
	if confirmStop {
		fmt.Print("\nОстановить ноду? Нажмите y для подтверждения или n для отмены")
	}
}

func isLocalEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func endpointPort(endpoint string) int {
	u, err := url.Parse(endpoint)
	if err != nil {
		return 8080
	}
	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil && port > 0 {
			return port
		}
	}
	if strings.EqualFold(u.Scheme, "https") {
		return 443
	}
	return 8080
}

func endpointWithPort(endpoint string, port int) string {
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" {
		return endpoint
	}
	u.Host = net.JoinHostPort(u.Hostname(), strconv.Itoa(port))
	return u.String()
}

func endpointUsesTLS(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && strings.EqualFold(u.Scheme, "https")
}

func nodeProcessSummary(endpoint, nodeBin string) string {
	if !isLocalEndpoint(endpoint) {
		return "удалённая нода; процессом управляют на удалённом хосте"
	}
	if nodeBin == "" {
		return "локальная нода; cerera не найден рядом, задайте --node-bin для запуска"
	}
	return "локальная нода; внешний процесс или остановлена"
}

func defaultNodeBinary() string {
	path, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "cerera"
	if filepath.Ext(path) != "" {
		name += filepath.Ext(path)
	}
	candidate := filepath.Join(filepath.Dir(path), name)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

func renderOverview(endpoint string) {
	height, info, err := queryNodeHealth(endpoint)
	if err != nil {
		fmt.Printf("Состояние: НОДА НЕДОСТУПНА\n\n%s\n", err)
		return
	}
	var fields map[string]any
	if err := json.Unmarshal(info, &fields); err != nil {
		fmt.Printf("Не удалось разобрать chain.getInfo: %s\n", err)
		return
	}
	fmt.Println("Состояние: подключено")
	fmt.Printf("Высота:       %d\n", height)
	fmt.Printf("Блоков всего: %v\n", valueOrDash(fields["total"]))
	fmt.Printf("Транзакций:   %v\n", valueOrDash(fields["txs"]))
	fmt.Printf("Интервал:     %v сек\n", valueOrDash(fields["avgTime"]))
	fmt.Printf("Размер цепи:  %v байт\n", valueOrDash(fields["size"]))
	fmt.Printf("Последний:    %s\n", valueOrDash(fields["latest"]))

	if raw, err := rpcCall(endpoint, "cerera.pool.getInfo", []any{}); err == nil {
		var pool map[string]any
		if json.Unmarshal(raw, &pool) == nil {
			fmt.Printf("Mempool:      %v транзакций\n", valueOrDash(pool["size"]))
		}
	} else {
		fmt.Printf("Mempool:      ошибка RPC (%s)\n", err)
	}
}

func renderLatestBlock(endpoint string) {
	height, err := rpcCall(endpoint, "cerera.chain.height", []any{})
	if err != nil {
		fmt.Printf("Не удалось получить высоту: %s\n", err)
		return
	}
	var total int
	if err := json.Unmarshal(height, &total); err != nil {
		fmt.Printf("Некорректная высота: %s\n", err)
		return
	}
	if total < 1 {
		fmt.Println("Цепь пока пуста")
		return
	}
	fmt.Printf("Последний блок (индекс %d)\n\n", total-1)
	renderRawJSON(endpoint, "cerera.chain.getBlockByIndex", []any{total - 1})
}

func renderJSONResult(endpoint, method, title string) {
	fmt.Printf("%s\n\n", title)
	renderRawJSON(endpoint, method, []any{})
}

func renderRawJSON(endpoint, method string, params []any) {
	raw, err := rpcCall(endpoint, method, params)
	if err != nil {
		fmt.Printf("RPC ошибка: %s\n", err)
		return
	}
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		fmt.Printf("Некорректный JSON: %s\n", err)
		return
	}
	formatted, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		fmt.Printf("Не удалось отформатировать ответ: %s\n", err)
		return
	}
	fmt.Println(string(formatted))
}

func valueOrDash(value any) any {
	if value == nil {
		return "—"
	}
	return value
}
