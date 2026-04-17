package cepheusagent

import (
	"cepheus/api"
	"context"
)

type Executor interface {
	Execute(ctx context.Context, params api.TaskParams, spec *api.Task) (api.ProbeResult, error)
}
