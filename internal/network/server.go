package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cerera/config"
	"github.com/cerera/internal/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var httpLogger = logger.Named("http")

func SetUpHttp(ctx context.Context, cfg *config.Config, port int) error {
	if cfg == nil {
		return fmt.Errorf("HTTP config is nil")
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/ws", HandleWebSockerRequest(ctx, cfg.SEC.HTTP.AllowedOrigins))
	mux.Handle("/", HandleRequest(ctx, cfg.SEC.HTTP.AllowedOrigins))

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("listen for HTTP RPC on port %d: %w", port, err)
	}
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if cfg.SEC.HTTP.TLS {
		cert, err := tls.LoadX509KeyPair("./server.crt", "./server.key")
		if err != nil {
			_ = listener.Close()
			return fmt.Errorf("load HTTPS certificate: %w", err)
		}
		server.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			httpLogger.Errorw("HTTP server graceful shutdown failed", "err", err)
			_ = server.Close()
		}
	}()
	go func() {
		var serveErr error
		if cfg.SEC.HTTP.TLS {
			serveErr = server.ServeTLS(listener, "", "")
		} else {
			serveErr = server.Serve(listener)
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			httpLogger.Errorw("HTTP server stopped unexpectedly", "err", serveErr)
		}
	}()
	httpLogger.Infow("HTTP server started", "port", port, "tls", cfg.SEC.HTTP.TLS)
	return nil
}
