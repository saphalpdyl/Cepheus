package agent

import (
	"cepheus/libs/common"
	"context"
)

type Executor interface {
	Execute(ctx context.Context, params TaskParams, spec *Task) (common.ProbeResult, error)
}
