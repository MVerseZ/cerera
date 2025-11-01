package network

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cerera/internal/cerera/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

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

	fmt.Printf("Starting http server at port %d\r\n", port)
	go http.HandleFunc("/", HandleRequest(ctx))
	go http.HandleFunc("/ws", HandleWebSockerRequest(ctx))

	return nil
}
