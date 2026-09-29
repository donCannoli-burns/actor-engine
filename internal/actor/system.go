package actor

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Message struct {
	Topic string
	Body  any
}

type Handler func(context.Context, Message) error

type ref struct {
	name    string
	mailbox chan Message
	handle  Handler
}

type System struct {
	mu     sync.RWMutex
	actors map[string]*ref
	wg     sync.WaitGroup
}

func NewSystem() *System {
	return &System{actors: make(map[string]*ref)}
}

func (s *System) Spawn(ctx context.Context, name string, handle Handler) error {
	if name == "" || handle == nil {
		return errors.New("actor name and handler are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.actors[name]; exists {
		return fmt.Errorf("actor %q already exists", name)
	}
	r := &ref{name: name, mailbox: make(chan Message, 1), handle: handle}
	s.actors[name] = r
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-r.mailbox:
				_ = r.handle(ctx, msg)
			}
		}
	}()
	return nil
}

func (s *System) Send(ctx context.Context, target string, msg Message) error {
	s.mu.RLock()
	r, ok := s.actors[target]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("actor %q not found", target)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r.mailbox <- msg:
		return nil
	}
}

func (s *System) Wait() {
	s.wg.Wait()
}
