package service

import (
	"context"
	"fmt"

	"github.com/cerera/config"
	"github.com/cerera/internal/logger"
)

var pipelinelogger = logger.Named("[pipeline]")

type Stage any

type Pipeline struct {
	ctx context.Context
	cfg *config.Config
	// define pipeline stages
	stages   []Stage
	checksum [32]byte
	registry *Registry
}

func SetupPipeline(ctx context.Context, cfg *config.Config, registry *Registry) error {
	p := &Pipeline{
		ctx:      ctx,
		cfg:      cfg,
		stages:   []Stage{},
		checksum: [32]byte{},
		registry: registry,
	}
	pipelinelogger.Info("Pipeline started")
	err := p.setupPipeline()
	if err != nil {
		return fmt.Errorf("failed to setup pipeline: %w", err)
	}
	p.loadStages()
	return nil
}

func (p *Pipeline) setupPipeline() error {
	return nil
}

func (p *Pipeline) loadStages() {
	// Load and initialize stages here
	pipelinelogger.Info("Loading pipeline stages")
	// Example: pipeline.Stages = append(pipeline.Stages, NewStage1(), NewStage2())
}
