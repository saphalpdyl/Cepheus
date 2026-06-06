package agent

import (
	"cepheus/api"
	"context"
	"time"
)

type RunningTask struct {
	Spec      *api.Task
	cancel    context.CancelFunc
	done      <-chan struct{}
	startedAt time.Time
	errors    int
}

func (rt *RunningTask) Stop() {
	rt.cancel()
	<-rt.done
}

func (rt *RunningTask) HotUpdate(newSpec api.Task) {
	// rt.Spec.Store(&newSpec)
}
