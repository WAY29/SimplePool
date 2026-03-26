package tunnel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/WAY29/SimplePool/internal/runtime/singbox"
)

type RuntimeManagerOptions struct {
	Compiler singbox.Compiler
	Factory  singbox.BoxFactory
	Now      func() time.Time
}

type managedRuntime struct {
	mu         sync.Mutex
	compiler   singbox.Compiler
	factory    singbox.BoxFactory
	now        func() time.Time
	processes  map[string]*singbox.Supervisor
	httpClient *time.Timer
}

func NewRuntimeManager(options RuntimeManagerOptions) RuntimeManager {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &managedRuntime{
		compiler:  options.Compiler,
		factory:   options.Factory,
		now:       now,
		processes: make(map[string]*singbox.Supervisor),
	}
}

func (m *managedRuntime) Start(ctx context.Context, tunnelID string, layout singbox.RuntimeLayout, config []byte) error {
	return m.supervisor(tunnelID).Start(ctx, singbox.StartRequest{
		Layout: layout,
		Config: config,
	})
}

func (m *managedRuntime) Stop(ctx context.Context, tunnelID string) error {
	m.mu.Lock()
	supervisor := m.processes[tunnelID]
	m.mu.Unlock()
	if supervisor == nil {
		return nil
	}
	return supervisor.Stop()
}

func (m *managedRuntime) Delete(ctx context.Context, tunnelID string) error {
	m.mu.Lock()
	supervisor := m.processes[tunnelID]
	delete(m.processes, tunnelID)
	m.mu.Unlock()
	if supervisor == nil {
		return nil
	}
	return supervisor.Stop()
}

func (m *managedRuntime) GetSelector(ctx context.Context, tunnelID string, controllerPort int, secret string) (*singbox.ProxyInfo, error) {
	client := singbox.NewClashAPIClient(fmt.Sprintf("http://127.0.0.1:%d", controllerPort), secret, nil)
	return client.GetProxy(ctx, selectorTag)
}

func (m *managedRuntime) SwitchSelector(ctx context.Context, tunnelID string, controllerPort int, secret, outbound string) error {
	client := singbox.NewClashAPIClient(fmt.Sprintf("http://127.0.0.1:%d", controllerPort), secret, nil)
	return client.SwitchSelector(ctx, selectorTag, outbound)
}

func (m *managedRuntime) Close() error {
	m.mu.Lock()
	supervisors := make([]*singbox.Supervisor, 0, len(m.processes))
	for _, item := range m.processes {
		supervisors = append(supervisors, item)
	}
	m.processes = make(map[string]*singbox.Supervisor)
	m.mu.Unlock()

	var errs []error
	for _, item := range supervisors {
		errs = append(errs, item.Stop())
	}
	return errors.Join(errs...)
}

func (m *managedRuntime) supervisor(tunnelID string) *singbox.Supervisor {
	m.mu.Lock()
	defer m.mu.Unlock()
	if supervisor := m.processes[tunnelID]; supervisor != nil {
		return supervisor
	}
	supervisor := singbox.NewSupervisor(singbox.SupervisorOptions{
		Compiler: m.compiler,
		Factory:  m.factory,
		Now:      m.now,
	})
	m.processes[tunnelID] = supervisor
	return supervisor
}
