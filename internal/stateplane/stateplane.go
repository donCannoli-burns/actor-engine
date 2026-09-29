package stateplane

import (
	"sort"
	"sync"
)

type Plane struct {
	mu     sync.RWMutex
	active map[string]struct{}
}

func New(initial ...string) *Plane {
	p := &Plane{active: make(map[string]struct{}, len(initial))}
	for _, state := range initial {
		p.active[state] = struct{}{}
	}
	return p
}

func (p *Plane) Activate(states ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, state := range states {
		p.active[state] = struct{}{}
	}
}

func (p *Plane) Deactivate(states ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, state := range states {
		delete(p.active, state)
	}
}

func (p *Plane) SetExclusive(group []string, active string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, state := range group {
		delete(p.active, state)
	}
	if active != "" {
		p.active[active] = struct{}{}
	}
}

func (p *Plane) Has(state string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.active[state]
	return ok
}

func (p *Plane) Snapshot() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, 0, len(p.active))
	for state := range p.active {
		out = append(out, state)
	}
	sort.Strings(out)
	return out
}
