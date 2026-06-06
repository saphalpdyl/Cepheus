package agent

import (
	"cepheus/api"
	"cepheus/libs/common"
	"context"
)

type Executor interface {
	Execute(ctx context.Context, params api.TaskParams, spec *api.Task) (common.ProbeResult, error)
}
