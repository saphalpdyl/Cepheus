package argus

import (
	"cepheus/argus/types"
	"context"
	"fmt"
	"log/slog"
)

// Employs the policy engine for reporting events
// Gathers different metrics (instant findings from detectors) and
// reports/alarms based on leaky bucket
// https://en.wikipedia.org/wiki/Leaky_bucket

type PolicyEngine struct {
	logger *slog.Logger
}

type PolicyEngineConfig struct {
}

func NewPolicyEngine(cfg PolicyEngineConfig, logger *slog.Logger) PolicyEngine {
	return PolicyEngine{
		logger: logger,
	}
}

func (p *PolicyEngine) ApplyFinding(ctx context.Context, finding *types.Finding) error {
	if finding != nil {
		p.logger.WarnContext(ctx, fmt.Sprintf("ANAMOLY @ %f with VALUE %f", finding.Severity, finding.Value))
	}

	return nil
}
