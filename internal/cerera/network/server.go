package network

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cerera/internal/cerera/config"
	"github.com/cerera/internal/cerera/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var httpLogger = logger.Named("http")

func SetUpHttp(ctx context.Context, cfg *config.Config, port int) error {
	rpcRequestMetric := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "rpc_requests_hits",
			Help: "Count http rpc requests",
		},
	)
	prometheus.MustRegister(rpcRequestMetric)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		if cfg.SEC.HTTP.TLS {
			err := http.ListenAndServeTLS(fmt.Sprintf(":%d", port), "./server.crt", "./server.key", nil)
			if err != nil {
				httpLogger.Errorw("Error starting HTTPS server", "err", err)
			}
		} else {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
				httpLogger.Errorw("Error starting HTTP server", "err", err)
			}
		}
	}()

	if cfg.SEC.HTTP.TLS {
		httpLogger.Infow("Starting HTTPS server", "port", port)
	} else {
		httpLogger.Infow("Starting HTTP server", "port", port)
	}
	go http.HandleFunc("/", HandleRequest(ctx))
	go http.HandleFunc("/ws", HandleWebSockerRequest(ctx))

	return nil
}
