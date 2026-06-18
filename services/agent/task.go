package agent

import (
	"context"
	"time"
)

type RunningTask struct {
	Spec      *Task
	cancel    context.CancelFunc
	done      <-chan struct{}
	startedAt time.Time
	errors    int
}

func (rt *RunningTask) Stop() {
	rt.cancel()
	<-rt.done
}

func (rt *RunningTask) HotUpdate(newSpec Task) {
	// rt.Spec.Store(&newSpec)
}
