package miner

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"sync"
	"time"

	"github.com/cerera/core/block"
	"github.com/cerera/core/common"
	"github.com/cerera/core/types"
	"github.com/cerera/internal/coinbase"
)

const cancelCheckInterval = 64

// Worker представляет собой воркер для долгих вычислений.
// stopCtx живёт всё время работы воркера; taskCancel отменяет только текущий поиск nonce,
// чтобы Update мог перезапустить майнинг с новым набором транзакций.
type Worker struct {
	mu          sync.Mutex
	stopCtx     context.Context
	stopCancel  context.CancelFunc
	taskCancel  context.CancelFunc
	currentTask *Task
	resultCh    chan Result
	jobID       uint64
	heightLock  HeightLockChecker
}

// Task представляет задачу для вычисления
type Task struct {
	id    types.Address
	chain int
	prev  *block.Block
	txs   []*types.GTransaction
}

// Result представляет результат вычисления
type Result struct {
	id      types.Address
	Value   *block.Block
	Error   error
	Elapsed time.Duration
	jobID   uint64
}

// NewWorker создает новый экземпляр воркера
func NewWorker() *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		stopCtx:    ctx,
		stopCancel: cancel,
		resultCh:   make(chan Result, 1),
	}
}

// Start запускает воркер. Майнинг стартует через Compute.
func (w *Worker) Start() {}

func (w *Worker) SetHeightLock(lock HeightLockChecker) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.heightLock = lock
}

// Compute запускает вычисление с новым параметром.
// Если вычисление уже выполняется, оно отменяется и запускается заново.
func (w *Worker) Compute(t *Task) {
	if t == nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.stopCtx.Err() != nil {
		return
	}

	if w.taskCancel != nil {
		w.taskCancel()
	}

	ctx, cancel := context.WithCancel(w.stopCtx)
	w.taskCancel = cancel
	w.jobID++
	jobID := w.jobID
	w.currentTask = t

	go w.mine(ctx, t, jobID)
}

// GetResult возвращает канал с результатами вычислений
func (w *Worker) GetResult() <-chan Result {
	return w.resultCh
}

// Stop останавливает воркер и текущее вычисление
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.taskCancel != nil {
		w.taskCancel()
	}
	if w.stopCancel != nil {
		w.stopCancel()
	}
}

// GetCurrentTask возвращает информацию о текущей задаче
func (w *Worker) GetCurrentTask() *Task {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentTask
}

func (w *Worker) isStale(jobID uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return jobID != w.jobID
}

func containsTx(txs []types.GTransaction, tx *types.GTransaction) bool {
	if tx == nil {
		return false
	}
	h := tx.Hash()
	for i := range txs {
		if txs[i].Hash() == h {
			return true
		}
	}
	return false
}

func mergeTx(existing []types.GTransaction, tx types.GTransaction) []types.GTransaction {
	if containsTx(existing, &tx) {
		return existing
	}
	out := make([]types.GTransaction, len(existing)+1)
	copy(out, existing)
	out[len(existing)] = tx
	return out
}

func remainingTxs(task *Task, mined *block.Block) []*types.GTransaction {
	if task == nil || len(task.txs) == 0 || mined == nil {
		return nil
	}
	included := make(map[common.Hash]struct{}, len(mined.Transactions))
	for i := range mined.Transactions {
		included[mined.Transactions[i].Hash()] = struct{}{}
	}
	out := make([]*types.GTransaction, 0, len(task.txs))
	for i := range task.txs {
		if _, ok := included[task.txs[i].Hash()]; !ok {
			out = append(out, task.txs[i])
		}
	}
	return out
}

// func (w *Worker) createNewBlock(lastBlock *block.Block, transactions []types.GTransaction) *block.Block {
func (w *Worker) createNewBlock(data *Task) *block.Block {

	transactions := data.txs

	lastBlock := data.prev
	lastHeader := data.prev.Header()
	if lastHeader == nil {
		minerLogger().Errorw("[MINER] createNewBlock: lastBlock header is nil")
		return nil
	}

	// Создаем заголовок нового блока
	newHeader := &block.Header{
		Ctx:        lastHeader.Ctx,
		Difficulty: lastHeader.Difficulty,
		Extra:      [8]byte{0x1, 0xf, 0x0, 0x0, 0x0, 0x0, 0xd, 0xe},
		Height:     lastHeader.Height + 1,
		Index:      lastHeader.Index + 1,
		GasLimit:   lastHeader.GasLimit,
		GasUsed:    0, // Будет рассчитано после обработки транзакций
		ChainId:    data.chain,
		Node:       data.id, // Адрес майнера
		Size:       0,       // Будет рассчитано после создания блока
		Timestamp:  uint64(time.Now().UnixMilli()),
		V:          [8]byte{0xe, 0x0, 0xf, 0xf, 0xf, 0xf, 0x2, 0x1},
		Nonce:      lastHeader.Nonce, // Будет установлен при майнинге
		PrevHash:   lastBlock.GetHash(),
		Root:       common.EmptyRootHash, // Будет рассчитан
	}

	// Создаем новый блок
	newBlock := block.NewBlock(newHeader)

	// Фильтруем транзакции по GasLimit: добавляем только те, которые помещаются в лимит
	var totalGasUsed uint64
	var selectedTxs []types.GTransaction

	for _, tx := range transactions {
		txType := tx.Type()
		txGas := tx.Gas()

		// Coinbase-транзакции из пула пропускаем: свежий coinbase создаётся ниже,
		// иначе в блок попадут два coinbase.
		if txType == types.CoinbaseTxType {
			continue
		}

		// Faucet-транзакции не учитываются в GasUsed, но добавляются в блок.
		if txType == types.FaucetTxType {
			selectedTxs = append(selectedTxs, *tx)
			continue
		}

		// Для обычных транзакций проверяем GasLimit.
		// Используем continue (не break): транзакции отсортированы по fee rate,
		// а не по газу, поэтому меньшая tx после крупной может ещё поместиться.
		if totalGasUsed+txGas > newHeader.GasLimit {
			minerLogger().Debugw("[MINER] Transaction exceeds gas limit, skipping",
				"tx_hash", tx.Hash(),
				"tx_gas", txGas,
				"current_gas_used", totalGasUsed,
				"gas_limit", newHeader.GasLimit)
			continue
		}
		selectedTxs = append(selectedTxs, *tx)
		totalGasUsed += txGas
	}

	// Добавляем отобранные транзакции в блок
	newBlock.Transactions = selectedTxs

	// Добавляем coinbase транзакцию (она не учитывается в GasUsed)
	coinbaaseTransaction := coinbase.CreateCoinBaseTransation(lastHeader.Nonce, data.id)
	newBlock.Transactions = append(newBlock.Transactions, coinbaaseTransaction)

	// Рассчитываем размер блока
	blockBytes := newBlock.ToBytes()
	newBlock.Head.Size = len(blockBytes)

	// Устанавливаем использованный газ (только для обычных транзакций, без coinbase и faucet)
	newBlock.Head.GasUsed = uint64(totalGasUsed)

	minerLogger().Debugw("[MINER] Block created",
		"height", newHeader.Height,
		"gas_used", totalGasUsed,
		"gas_limit", newHeader.GasLimit,
		"txs_count", len(selectedTxs),
		"total_txs_in_block", len(newBlock.Transactions))

	return newBlock
}

// doLongComputation выполняет долгое вычисление
func (w *Worker) mine(ctx context.Context, task *Task, jobID uint64) {
	startTime := time.Now()

	if task == nil || task.prev == nil || task.prev.Head == nil {
		minerLogger().Errorw("[MINER] invalid mining task")
		return
	}

	w.mu.Lock()
	heightLock := w.heightLock
	w.mu.Unlock()
	nextHeight := task.prev.Head.Height + 1
	if heightLock != nil && heightLock.IsHeightLocked(nextHeight) {
		minerLogger().Debugw("[MINER] skip mining locked height", "height", nextHeight)
		return
	}

	minerLogger().Infow("[MINER] 🚀 Started mining ", "task id", task.id, "task txs", len(task.txs))

	var block = w.createNewBlock(task)
	if block == nil {
		minerLogger().Errorw("[MINER] failed to create block")
		return
	}

	select {
	case <-ctx.Done():
		minerLogger().Debugw("[MINER] mining cancelled before nonce search")
		return
	default:
		// Обновляем метрику difficulty
		minerCurrentDifficulty.Set(float64(block.Header().Difficulty))

		// Рассчитываем хеш блока
		blockHash, err := block.CalculateHash()
		if err != nil {
			minerLogger().Errorw("Error calculating block hash", "err", err, "height", block.Header().Height)
			return
		}

		// Защита от деления на ноль
		if block.Header().Difficulty == 0 {
			return
		}
		target := new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(int64(block.Header().Difficulty)))

		// Обновляем метрику target (используем логарифм для больших чисел)
		targetLog := 256*math.Log2(2) - math.Log2(float64(block.Header().Difficulty))
		minerCurrentTarget.Set(targetLog)

		block.Hash = common.BytesToHash(blockHash)
		blockHashInt := new(big.Int).SetBytes(blockHash)

		// Вычисляем отношение хэша к target для мониторинга
		hashBitLen := blockHashInt.BitLen()
		hashLog := float64(hashBitLen)
		ratio := hashLog - targetLog
		minerHashToTargetRatio.Set(ratio)

		// Проверяем начальный хэш
		minerHashValidationTotal.Inc()
		if blockHashInt.Cmp(target) < 0 {
			minerHashValidTotal.Inc()
		} else {
			minerHashInvalidTotal.Inc()
		}

		// Начинаем отслеживание поиска nonce
		nonceSearchStartTime := time.Now()
		nonceSearchAttempts := uint64(0)

		// Цикл поиска валидного nonce: продолжаем пока хеш >= target (невалидный)
		for blockHashInt.Cmp(target) >= 0 {
			if nonceSearchAttempts%cancelCheckInterval == 0 {
				select {
				case <-ctx.Done():
					minerLogger().Debugw("[MINER] mining cancelled during nonce search",
						"height", block.Header().Height,
						"attempts", nonceSearchAttempts)
					return
				default:
				}
			}

			// Increment nonce directly on the header pointer (block.Header() returns
			// block.Head itself, not a copy, so assigning back would be a no-op).
			// Detect uint64 overflow: if nonce wraps to 0 we have exhausted the space.
			next, carry := bits.Add64(block.Head.Nonce, 1, 0)
			if carry != 0 {
				return
			}
			block.Head.Nonce = next
			newBlockHash, _ := block.CalculateHash()
			blockHash = newBlockHash
			blockHashInt = new(big.Int).SetBytes(newBlockHash)

			// Обновляем метрики поиска nonce
			nonceSearchAttempts++
			minerNonceSearchAttemptsTotal.Inc()

			// Обновляем отношение хэша к target (используем логарифм через bitLen)
			hashBitLen = blockHashInt.BitLen()
			hashLog = float64(hashBitLen)
			ratio = hashLog - targetLog
			minerHashToTargetRatio.Set(ratio)

			// Проверяем хэш на каждой итерации
			minerHashValidationTotal.Inc()
			if blockHashInt.Cmp(target) < 0 {
				minerHashValidTotal.Inc()
			} else {
				minerHashInvalidTotal.Inc()
			}
		}

		// Обновляем метрики после нахождения валидного nonce
		nonceSearchDuration := time.Since(nonceSearchStartTime).Seconds()
		minerNonceSearchAttempts.Observe(float64(nonceSearchAttempts))
		minerNonceSearchDurationSeconds.Observe(nonceSearchDuration)

		// Обновляем block.Hash после нахождения валидного хеша
		block.Hash = common.BytesToHash(blockHash)

		minerLogger().Infow("[MINER] Block mined",
			"height", block.Header().Height,
			"nonce", block.Header().Nonce,
			"hash", fmt.Sprintf("%x", blockHash))

	}

	if ctx.Err() != nil {
		return
	}

	elapsed := time.Since(startTime)

	select {
	case <-ctx.Done():
		minerLogger().Debugw("[MINER] mining cancelled while sending result")
	case w.resultCh <- Result{
		Value:   block,
		Error:   nil,
		Elapsed: elapsed,
		jobID:   jobID,
	}:
		fmt.Printf("✅ Mining completed! Result: %s (time: %v)\n",
			block.Hash, elapsed)
	}
}
