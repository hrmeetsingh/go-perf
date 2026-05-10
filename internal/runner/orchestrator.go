package runner

import (
	"context"
	"fmt"
	"sync"
)

type Orchestrator struct {
	config OrchestratorConfig
	auth   map[string]string
	engine Engine
}

func NewOrchestrator(config OrchestratorConfig, authMetadata map[string]string, engine Engine) *Orchestrator {
	return &Orchestrator{
		config: config,
		auth:   authMetadata,
		engine: engine,
	}
}

func (o *Orchestrator) Run(ctx context.Context) (*MultiVariantResult, error) {
	if err := o.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid orchestrator config: %w", err)
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []*Result
		errs    []error
	)

	for _, variant := range o.config.Variants {
		wg.Add(1)
		go func(v VariantConfig) {
			defer wg.Done()

			result, err := o.engine.Run(ctx,
				o.config.RunConfig.Call,
				o.config.RunConfig.Target,
				o.config.RunConfig,
				v,
				o.auth,
			)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("variant %q: %w", v.Name, err))
				return
			}
			results = append(results, result)
		}(variant)
	}

	wg.Wait()

	if len(errs) > 0 {
		return nil, fmt.Errorf("variant errors: %v", errs)
	}

	return &MultiVariantResult{Results: results}, nil
}

func (o *Orchestrator) RunSingle(ctx context.Context, payload map[string]interface{}) (*Result, error) {
	variant := VariantConfig{
		Name:    "default",
		Payload: payload,
	}
	return o.engine.Run(ctx,
		o.config.RunConfig.Call,
		o.config.RunConfig.Target,
		o.config.RunConfig,
		variant,
		o.auth,
	)
}
