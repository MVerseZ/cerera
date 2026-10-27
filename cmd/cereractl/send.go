package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cerera/config"
	"github.com/cerera/core/address"
	"github.com/cerera/core/crypto"
	"github.com/cerera/core/types"
	"github.com/cerera/pallada"
)

func runSend(args []string) {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	toFlag := fs.String("to", "", "Recipient address (required)")
	amountFlag := fs.String("amount", "", "Amount in CER, up to 6 decimal places (required)")
	message := fs.String("message", "", "Optional transaction message")
	gas := fs.Uint64("gas", 21000, "Gas limit")
	httpPort := fs.Int("http", 0, "Node HTTP RPC port (defaults to 8080 or the endpoint port)")
	endpointDefault := os.Getenv("CERERA_RPC_URL")
	if endpointDefault == "" {
		endpointDefault = "http://127.0.0.1:8080/"
	}
	endpoint := fs.String("endpoint", endpointDefault, "Node JSON-RPC endpoint")
	nodeKey := fs.String("node-key", "ddddd.nodekey.pem", "Local node private key PEM file")
	dataDir := fs.String("data-dir", defaultDataDir(), "Directory containing the node config.json")
	yes := fs.Bool("yes", false, "Send without interactive confirmation")
	if err := fs.Parse(args); err != nil {
		return
	}
	if *httpPort < 0 || *httpPort > 65535 || (hasFlag(args, "-http", "--http") && *httpPort == 0) {
		fmt.Fprintln(os.Stderr, "send: --http must be between 1 and 65535")
		os.Exit(2)
	}
	if *httpPort > 0 && !hasFlag(args, "-endpoint", "--endpoint") {
		*endpoint = endpointWithPort(*endpoint, *httpPort)
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "send: unexpected positional arguments")
		os.Exit(2)
	}
	if *toFlag == "" || *amountFlag == "" {
		fmt.Fprintln(os.Stderr, "send: --to and --amount are required")
		fs.Usage()
		os.Exit(2)
	}
	prepared, err := prepareNodeTransaction(*endpoint, *nodeKey, *dataDir, *toFlag, *amountFlag, *message, *gas)
	if err != nil {
		fmt.Fprintf(os.Stderr, "send: %v\n", err)
		os.Exit(1)
	}

	if !*yes {
		printPreparedTransaction(prepared, *endpoint)
		answer, err := readTextLine()
		if err != nil {
			fmt.Fprintln(os.Stderr, "\nsend: confirmation input unavailable; pass --yes to confirm explicitly")
			os.Exit(2)
		}
		if !strings.EqualFold(strings.TrimSpace(answer), "y") {
			fmt.Println("Отправка отменена")
			return
		}
	}

	txHash, err := submitPreparedTransaction(*endpoint, prepared)
	if err != nil {
		fmt.Fprintf(os.Stderr, "send failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Transaction queued\n  from: %s\n  to:   %s\n  hash: %s\n", prepared.from.Hex(), prepared.to.Hex(), txHash)
}

type preparedNodeTransaction struct {
	from    address.Address
	to      address.Address
	amount  string
	gas     uint64
	nonce   uint64
	chainID int
	signed  *types.GTransaction
}

func prepareNodeTransaction(endpoint, nodeKeyPath, dataDir, toText, amountText, message string, gas uint64) (*preparedNodeTransaction, error) {
	if !address.IsHexAddress(toText) {
		return nil, fmt.Errorf("invalid recipient address %q", toText)
	}
	if gas == 0 {
		return nil, fmt.Errorf("gas limit must be greater than zero")
	}
	if len(message) > 1024 {
		return nil, fmt.Errorf("message must be at most 1024 bytes")
	}
	amount, err := parseCERAmount(amountText)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	cfg, err := config.ReadConfig(filepath.Join(dataDir, config.ConfigFileName))
	if err != nil {
		return nil, fmt.Errorf("read node config: %w", err)
	}
	if cfg.Chain.ChainID <= 0 {
		return nil, fmt.Errorf("node config has an invalid chain ID")
	}
	keyPEM, err := os.ReadFile(nodeKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read node key %q: %w", nodeKeyPath, err)
	}
	privateKey, err := crypto.DecodePrivKey(string(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("invalid node key: %w", err)
	}
	from := crypto.PubkeyToAddress(&privateKey.PublicKey)
	if from != cfg.NetCfg.ADDR {
		return nil, fmt.Errorf("key address %s does not match configured node address %s", from.Hex(), cfg.NetCfg.ADDR.Hex())
	}
	if _, _, err := queryNodeHealth(endpoint); err != nil {
		return nil, fmt.Errorf("node RPC unavailable at %s: %w", endpoint, err)
	}
	nonceRaw, err := rpcCall(endpoint, "cerera.transaction.getNextNonce", []any{from.Hex()})
	if err != nil {
		return nil, fmt.Errorf("unable to get next node nonce: %w", err)
	}
	var nonce uint64
	if err := json.Unmarshal(nonceRaw, &nonce); err != nil {
		return nil, fmt.Errorf("invalid nonce response: %w", err)
	}
	chainID := big.NewInt(int64(cfg.Chain.ChainID))
	gasPrice := types.FloatToBigInt(pallada.MinGasPrice())
	to := address.HexToAddress(toText)
	unsigned, err := types.CreateUnbroadcastTransactionDust(nonce, to, amount, gas, gasPrice, message)
	if err != nil {
		return nil, fmt.Errorf("build transaction: %w", err)
	}
	signed, err := types.SignTx(unsigned, types.NewSimpleSigner(chainID), privateKey)
	if err != nil {
		return nil, fmt.Errorf("sign transaction: %w", err)
	}
	return &preparedNodeTransaction{from: from, to: to, amount: amountText, gas: gas, nonce: nonce, chainID: cfg.Chain.ChainID, signed: signed}, nil
}

func printPreparedTransaction(prepared *preparedNodeTransaction, endpoint string) {
	fmt.Printf("Отправить %s CER?\n  От: %s\n  Кому: %s\n  Gas limit: %d\n  Nonce: %d\n  Chain ID: %d\n  RPC: %s\nПодтвердить? [y/N] ", prepared.amount, prepared.from.Hex(), prepared.to.Hex(), prepared.gas, prepared.nonce, prepared.chainID, endpoint)
}

func submitPreparedTransaction(endpoint string, prepared *preparedNodeTransaction) (string, error) {
	encoded, err := json.Marshal(prepared.signed)
	if err != nil {
		return "", fmt.Errorf("encode signed transaction: %w", err)
	}
	result, err := rpcCall(endpoint, "cerera.pool.sendRawTransaction", []any{string(encoded)})
	if err != nil {
		return "", err
	}
	var txHash string
	if err := json.Unmarshal(result, &txHash); err != nil {
		return "", fmt.Errorf("unexpected RPC result: %s", result)
	}
	return txHash, nil
}

func sendFromTUI(endpoint, nodeKeyPath, dataDir string) string {
	to, err := promptText("Получатель (hex): ")
	if err != nil {
		return "Не удалось прочитать адрес: " + err.Error()
	}
	amount, err := promptText("Сумма CER (до 6 знаков после точки): ")
	if err != nil {
		return "Не удалось прочитать сумму: " + err.Error()
	}
	message, err := promptText("Сообщение (Enter — без сообщения): ")
	if err != nil {
		return "Не удалось прочитать сообщение: " + err.Error()
	}
	gasText, err := promptText("Gas limit [21000]: ")
	if err != nil {
		return "Не удалось прочитать gas limit: " + err.Error()
	}
	gas := uint64(21000)
	if strings.TrimSpace(gasText) != "" {
		gas, err = strconv.ParseUint(strings.TrimSpace(gasText), 10, 64)
		if err != nil {
			return "Некорректный gas limit"
		}
	}
	prepared, err := prepareNodeTransaction(endpoint, nodeKeyPath, dataDir, strings.TrimSpace(to), strings.TrimSpace(amount), message, gas)
	if err != nil {
		return "Не удалось подготовить транзакцию: " + err.Error()
	}
	printPreparedTransaction(prepared, endpoint)
	answer, err := readTextLine()
	if err != nil {
		return "Не удалось прочитать подтверждение: " + err.Error()
	}
	if !strings.EqualFold(strings.TrimSpace(answer), "y") {
		return "Отправка отменена"
	}
	hash, err := submitPreparedTransaction(endpoint, prepared)
	if err != nil {
		return "Ошибка отправки: " + err.Error()
	}
	return fmt.Sprintf("Транзакция добавлена в mempool: %s", hash)
}

func promptText(prompt string) (string, error) {
	fmt.Print(prompt)
	return readTextLine()
}

func defaultDataDir() string {
	if dir := os.Getenv("CERERA_DATA_DIR"); dir != "" {
		return dir
	}
	return "."
}

func parseCERAmount(value string) (*big.Int, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return nil, fmt.Errorf("use a positive decimal with up to 6 fractional digits")
	}
	for _, part := range parts {
		for _, char := range part {
			if char < '0' || char > '9' {
				return nil, fmt.Errorf("use a positive decimal with up to 6 fractional digits")
			}
		}
	}
	if len(parts) == 2 && len(parts[1]) > 6 {
		return nil, fmt.Errorf("amount supports at most 6 fractional digits")
	}
	whole, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, fmt.Errorf("invalid whole amount")
	}
	fracText := ""
	if len(parts) == 2 {
		fracText = parts[1]
	}
	fracText += strings.Repeat("0", 6-len(fracText))
	frac := new(big.Int)
	if fracText != "" {
		if _, ok := frac.SetString(fracText, 10); !ok {
			return nil, fmt.Errorf("invalid fractional amount")
		}
	}
	whole.Mul(whole, big.NewInt(1_000_000))
	whole.Add(whole, frac)
	if whole.Sign() <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	if whole.BitLen() > 256 {
		return nil, fmt.Errorf("amount is too large")
	}
	return whole, nil
}
