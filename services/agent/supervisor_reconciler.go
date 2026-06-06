package agent

import (
	"context"
	"sync"
)

func (s *Supervisor) reconcile(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop removed tasks
	toStop := make([]*RunningTask, 0, len(s.running))

	for id, running := range s.running {
		if _, ok := s.desired[id]; !ok {
			toStop = append(toStop, running)
		}
	}

	var wg sync.WaitGroup
	wg.Add(len(toStop))
	for _, rt := range toStop {
		go func() {
			defer wg.Done()
			rt.Stop()
		}()
	}
	wg.Wait()

	for _, rt := range toStop {
		delete(s.running, rt.Spec.TaskID)
	}

	if len(toStop) > 0 {
		s.logger.InfoContext(ctx, "finished removing undesired tasks", "task_count", len(toStop))
	}

	for id, desired := range s.desired {
		running, exists := s.running[id]

		if !exists {
			// New task — start it
			rt := s.startTask(&desired)
			s.running[id] = rt
			continue
		}

		if running.Spec.Generation == desired.Generation {
			continue
		}

		// Generation changed — decide: hot update or restart
		// just restart for now
		s.logger.InfoContext(ctx, "restarting task", "task_id", running.Spec.TaskID)
		running.Stop()
		newRt := s.startTask(&desired)
		s.running[id] = newRt
	}
}
