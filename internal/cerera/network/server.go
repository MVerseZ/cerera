package network

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/cerera/internal/cerera/block"
	"github.com/cerera/internal/cerera/common"
	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/storage"
	"github.com/cerera/internal/cerera/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Peer struct {
	conn net.Conn
}

var (
	peers      []*Peer
	peersMutex sync.Mutex
	consensus  *block.Block
	votes      map[common.Hash]int // Карта для хранения голосов
	votesMutex sync.Mutex
)

var N *Node

func NewServer(cfg *config.Config, flag string, address string) {
	// Флаги
	// mode := flag.String("mode", "server", "Режим работы: server или client")
	// address := flag.String("address", "127.0.0.1:8080", "Адрес для подключения или прослушивания")
	// flag.Parse()
	// go SetUpHttp(httpPort)

	N = NewNode(cfg)
	N.Start()

	switch flag {
	case "server":
		go runServer(cfg.NetCfg.ADDR, address)
	case "client":
		go runClient(address)
	default:
		fmt.Println("Неизвестный режим. Используйте 'server' или 'client'.")
	}
}

// Запуск сервера
func runServer(cAddr types.Address, address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Ошибка при запуске сервера:", err)
		return
	}
	defer listener.Close()
	fmt.Printf("Сервер запущен на %s\n", address)

	// Принимаем входящие соединения
	acceptConnections(listener)

	// Чтение ввода с консоли для предложения значения
	// scanner := bufio.NewScanner(os.Stdin)
	// for scanner.Scan() {
	// 	proposedValue := scanner.Text()
	// 	fmt.Printf("Предложено значение: %s\n", proposedValue)
	// 	var b = block.GenerateGenesis(cAddr)
	// 	go startConsensus(b)
	// }

}

func Broadcast(msg []byte) {
	for _, peer := range peers {
		//_, err :=
		peer.conn.Write(msg)
		_, err := peer.conn.Write([]byte{'\n'})
		if err != nil {
			fmt.Println("Ошибка при отправке bradcast сообщения:", err)
		}
	}
}

// Принятие входящих соединений
func acceptConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			// fmt.Println("Ошибка при принятии соединения:", err)
			continue
		}
		fmt.Println("Новое соединение:", conn.RemoteAddr())

		peer := &Peer{conn: conn}
		peersMutex.Lock()
		peers = append(peers, peer)
		peersMutex.Unlock()

		go handlePeer(peer)
	}
}

// Обработка сообщений от пира
func handlePeer(peer *Peer) {
	defer peer.conn.Close()

	reader := bufio.NewReader(peer.conn)
	for {
		message, err := reader.ReadBytes('\n')
		// message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Соединение закрыто:", peer.conn.RemoteAddr())
			removePeer(peer)
			return
		}

		// message = strings.TrimSpace(message)
		// fmt.Printf("Recieve message: %s", message)
		// header, payload, _ := SplitMsg(message)
		// fmt.Println(header)
		// fmt.Println(payload)
		// var request RequestMsg
		// err = json.Unmarshal(payload, &request)
		// if err != nil {
		// 	fmt.Printf("error happened:%v", err)
		// 	return
		// }
		// fmt.Println(request)

		N.msgQueue <- message

		// b, err := block.FromBytes(message)
		// if err != nil {
		// 	panic(err)
		// }

		// if strings.HasPrefix(message, "vote:") {
		// Обработка голоса
		// value := strings.TrimPrefix(message, "vote:")
		// fmt.Printf("Получен голос от %s: %s\n", peer.conn.RemoteAddr(), b.Hash())
		// fmt.Printf("Теперь потдверждений у %s: %d\n", b.Hash(), b.Confirmations)
		// processVote(b)
		// } else {
		// 	fmt.Printf("Сообщение от %s: %s\n", peer.conn.RemoteAddr(), message)
		// }
	}
}

// Удаление пира
func removePeer(peer *Peer) {
	peersMutex.Lock()
	defer peersMutex.Unlock()

	for i, p := range peers {
		if p == peer {
			peers = append(peers[:i], peers[i+1:]...)
			break
		}
	}
	fmt.Println("Пир отключен:", peer.conn.RemoteAddr())
}

// Начало консенсуса
func startConsensus(proposedValue *block.Block) {
	peersMutex.Lock()
	defer peersMutex.Unlock()

	// Инициализация карты голосов vse v bloke (CONF)
	// remake
	proposedValue.Confirmations += 1
	votesMutex.Lock()
	var votes = make(map[common.Hash]int)
	votes[proposedValue.Hash()] = 1 // Голос сервера
	votesMutex.Unlock()

	fmt.Println("Начало голосования за значение:", proposedValue.Hash())

	// Отправка предложения всем пирам
	for _, peer := range peers {
		//_, err :=
		peer.conn.Write(proposedValue.ToBytes())
		_, err := peer.conn.Write([]byte{'\n'})
		if err != nil {
			fmt.Println("Ошибка при отправке предложения:", err)
		}
	}

	// Ожидание голосов
	time.Sleep(3 * time.Second) // Упрощенное ожидание

	// Подсчет голосов
	// votesMutex.Lock()
	maxVotes := 0
	// var chosenValue common.Hash
	// for value, count := range votes {
	// 	if count > maxVotes {
	// 		maxVotes = count
	// 		chosenValue = value
	// 	}
	// }
	// votesMutex.Unlock()

	// Принятие решения
	if maxVotes > len(peers)/2 {
		// consensus = chosenValue
		fmt.Printf("Консенсус достигнут: %s (голосов: %d)\n", consensus.Hash(), maxVotes)
	} else {
		fmt.Println("Консенсус не достигнут")
	}
}

// Обработка голоса
func processVote(b *block.Block) {
	votesMutex.Lock()
	defer votesMutex.Unlock()

	// Увеличиваем счетчик голосов для данного значения
	// votes[b.Hash()]++
}

// Запуск клиента
func runClient(address string) {
	var vlt = storage.GetVault()
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Ошибка при подключении к серверу:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Подключено к серверу", address)

	peer := &Peer{conn: conn}
	peersMutex.Lock()
	peers = append(peers, peer)
	peersMutex.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)

	// Горутина для чтения сообщений от сервера
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(conn)
		for {
			message, err := reader.ReadBytes('\n')

			// message, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Соединение с сервером разорвано")
				return
			}

			// fmt.Printf("Сообщение от сервера: %s\r\n", message)

			N.msgQueue <- message

			// message = strings.TrimSpace(message)
			// b, err := block.FromBytes(message)
			// if err != nil {
			// 	panic(err)
			// }
			// fmt.Printf("Сообщение от сервера: %s, подтверждений %d\n", b.Hash(), b.Confirmations)
			// b.Confirmations += 1
			// conn.Write(b.ToBytes())
			// conn.Write([]byte{'\n'})
			// fmt.Printf("Отправлено обратно сообщение: %s, подтверждений %d\n", b.Hash(), b.Confirmations)

			// if strings.HasPrefix(message, "propose:") {
			// 	// Получено предложение для голосования

			// 	value := strings.TrimPrefix(message, "propose:")
			// 	fmt.Printf("Получено предложение: %s\n", value)
			// 	if value == "bye" {
			// 		conn.Write([]byte("OP_NEG" + "\n"))
			// 	} else {
			// 		// Голосование (в данном примере всегда соглашаемся)
			// 		conn.Write([]byte("vote:" + value + "\n"))
			// 	}
			// } else {
			// 	fmt.Printf("Сообщение от сервера: %s\n", message)
			// }
		}
	}()

	// Горутина для отправки сообщений на сервер
	go func() {
		defer wg.Done()

		nodeSA := vlt.GetOwner()

		time.Sleep(2 * time.Second)
		var msg = types.HexToAddress("0xffFffffff00000000000000000000557D0b284521d71a7fca1e1C3F289849989e80B0b8100000000000000000aaaaaaa")
		var reqHeader = hJoin
		var handshakeReq = ComposeMsg(
			reqHeader,
			&JoinMsg{
				"join",
				int(time.Now().Unix()),
				N.NodeID,
				Request{
					msg.String(),
					hex.EncodeToString(generateDigest(msg)),
				},
				nodeSA.Bytes(),
			},
			[]byte{},
		)
		conn.Write(handshakeReq)
		conn.Write([]byte("\n"))

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			message := scanner.Text()
			_, err := conn.Write([]byte(message + "\n"))
			if err != nil {
				fmt.Println("Ошибка при отправке сообщения:", err)
				return
			}
		}
	}()

	wg.Wait()
}

func AddressToNodeId(addr types.Address) int {
	var baddr = addr.Bytes()
	var bgaddr = big.NewInt(0).SetBytes(baddr)
	return int(bgaddr.Int64())
}

func SetUpHttp(port int) {
	rpcRequestMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rpc_requests_hits",
			Help: "Count http rpc requests",
		},
	)
	prometheus.MustRegister(rpcRequestMetric)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		// if cfg.SEC.HTTP.TLS {
		// 	err := http.ListenAndServeTLS(fmt.Sprintf(":%d", port), "./server.crt", "./server.key", nil)
		// 	if err != nil {
		// 		fmt.Println("ListenAndServe: ", err)
		// 	}
		// } else {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			fmt.Println("Error starting server:", err)
		}
		// }
	}()

	var ctx = context.TODO()
	defer ctx.Done()

	fmt.Printf("Starting http server at port %d\r\n", port)
	go http.HandleFunc("/", HandleRequest(ctx))
	go http.HandleFunc("/ws", HandleWebSockerRequest(ctx))
}
