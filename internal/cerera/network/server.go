package network

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/cerera/internal/cerera/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var httpLogger = log.New(os.Stdout, "[http] ", log.LstdFlags|log.Lmicroseconds)

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
				httpLogger.Println("Error starting HTTPS server:", err)
			}
		} else {
			if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
				httpLogger.Println("Error starting HTTP server:", err)
			}
		}
	}()

	if cfg.SEC.HTTP.TLS {
		httpLogger.Printf("Starting HTTPS server at port %d\r\n", port)
	} else {
		httpLogger.Printf("Starting HTTP server at port %d\r\n", port)
	}
	go http.HandleFunc("/", HandleRequest(ctx))
	go http.HandleFunc("/ws", HandleWebSockerRequest(ctx))

	return nil
}
