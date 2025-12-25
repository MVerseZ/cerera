package icenet

import (
	"context"

	"github.com/cerera/internal/cerera/config"
)

// Start инициализирует и запускает компонент Ice.
// Создает новый экземпляр Ice с заданной конфигурацией и портом,
// затем запускает его. Возвращает указатель на Ice и ошибку, если инициализация не удалась.
//
// Параметры:
//   - cfg: конфигурация приложения, содержащая сетевые настройки
//   - ctx: контекст для управления жизненным циклом компонента
//   - port: порт для прослушивания входящих подключений
//
// Пример использования:
//
//	ice, err := icenet.Start(cfg, ctx, "31100")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer ice.Stop()
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
