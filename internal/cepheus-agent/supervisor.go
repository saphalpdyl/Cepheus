package cepheusagent

import "sync"

type Supervisor struct {
	scamper *Scamper
	mu      sync.RWMutex
}

func NewSupervisor() *Supervisor {
	return &Supervisor{}
}
