package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config describes basic logger options.
type Config struct {
	Path    string
	Level   string
	Console bool
}

var (
	globalLogger *zap.Logger
	loggerOnce   sync.Once
	loggerErr    error

	mu      sync.Mutex
	closers []io.Closer
)

// Init builds the global zap logger once and wires stdlib logging into it.
func Init(cfg Config) (*zap.Logger, error) {
	loggerOnce.Do(func() {
		var c []io.Closer
		var l *zap.Logger
		l, c, loggerErr = newLogger(cfg)
		if loggerErr != nil {
			return
		}
		globalLogger = l
		mu.Lock()
		closers = append(closers, c...)
		mu.Unlock()
		zap.ReplaceGlobals(globalLogger)
		_ = zap.RedirectStdLog(globalLogger)
	})
	return globalLogger, loggerErr
}

// L returns the global zap logger if initialised, otherwise zap.L().
func L() *zap.Logger {
	if globalLogger != nil {
		return globalLogger
	}
	return zap.L()
}

// Sugar returns a sugared logger backed by the global zap logger.
func Sugar() *zap.SugaredLogger {
	return L().Sugar()
}

// Named returns a sugared logger annotated with the provided name.
func Named(name string) *zap.SugaredLogger {
	return L().Named(name).Sugar()
}

// Sync flushes the logger buffers and closes any opened sinks.
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
	mu.Lock()
	defer mu.Unlock()
	for _, closer := range closers {
		_ = closer.Close()
	}
	closers = nil
}

func newLogger(cfg Config) (*zap.Logger, []io.Closer, error) {
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var sinks []zapcore.WriteSyncer
	var closerList []io.Closer

	if cfg.Console {
		sinks = append(sinks, zapcore.Lock(os.Stdout))
	}

	if cfg.Path != "" {
		file, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("open log file: %w", err)
		}
		sinks = append(sinks, zapcore.AddSync(file))
		closerList = append(closerList, file)
	}

	if len(sinks) == 0 {
		sinks = append(sinks, zapcore.Lock(os.Stdout))
	}

	levelText := strings.TrimSpace(strings.ToLower(cfg.Level))
	if levelText == "" {
		levelText = "info"
	}
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(levelText)); err != nil {
		level.SetLevel(zap.InfoLevel)
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.NewMultiWriteSyncer(sinks...),
		level,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return logger, closerList, nil
}
