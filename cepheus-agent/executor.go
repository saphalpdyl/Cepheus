package cepheusagent

import (
	"cepheus/api"
	"cepheus/common"
	"context"
)

type Executor interface {
	Execute(ctx context.Context, params api.TaskParams, spec *api.Task) (common.ProbeResult, error)
}
