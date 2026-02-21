package miner

import (
	"context"
	"fmt"
	"time"

	"github.com/cerera/core/types"
)

type Worker struct {
	ctx    context.Context
	cancel context.CancelFunc
	data   chan *types.GTransaction
}

func NewWorker() *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		ctx:    ctx,
		cancel: cancel,
		data:   make(chan *types.GTransaction, 10),
	}
}

func (w *Worker) Start() {
	go func() {
		for {
			select {
			case <-w.ctx.Done():
				fmt.Println("Горутина завершена")
				close(w.data)
				return
			// case d := <-w.data:
			// fmt.Printf("Обработаны данные: %d\n", d)
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (w *Worker) Process(data *types.GTransaction) error {
	select {
	case <-w.ctx.Done():
		return fmt.Errorf("worker stopped")
	case w.data <- data:
		// fmt.Printf("Обработаны данные: %d\n", data)
		return nil
	}
}

func (w *Worker) Stop() {
	w.cancel()
}
