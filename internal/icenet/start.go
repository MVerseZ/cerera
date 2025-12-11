package icenet

import (
	"context"

	"github.com/cerera/internal/cerera/config"
)

// Start инициализирует и запускает компонент Ice
func Start(cfg *config.Config, ctx context.Context, port string) (*Ice, error) {
	ice, err := NewIce(cfg, ctx, port)
	if err != nil {
		return nil, err
	}

	if err := ice.Start(); err != nil {
		return nil, err
	}

	return ice, nil
}
