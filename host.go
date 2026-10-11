// Copyright 2026 dywoq - Apache License 2.0
// https://github.com/dywoq/scintilla

package scintilla

import (
	"context"
	"iter"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type HostModuleFunction struct {
	Name    string
	GoFunc  api.GoModuleFunc
	Params  []api.ValueType
	Results []api.ValueType
}

// HostModuleRegistry is a centralized storage, which registers provided host modules
// to [wazero.Runtime]. It is used by [Script] instances. It is thread safe.
type HostModuleRegistry struct {
	modules []api.Module
	mu      sync.Mutex
	runtime wazero.Runtime
	ctx     context.Context
}

func NewHostModuleFunction(name string, goFunc api.GoModuleFunc, params []api.ValueType, results []api.ValueType) *HostModuleFunction {
	return &HostModuleFunction{
		Name:    name,
		GoFunc:  goFunc,
		Params:  params,
		Results: results,
	}
}

// NewHostModuleRegistry pre-allocates the underlying modules slice with the capacity as 4.
func NewHostModuleRegistry(ctx context.Context) *HostModuleRegistry {
	return &HostModuleRegistry{
		ctx:     ctx,
		modules: make([]api.Module, 0, 4),
	}
}

// RegisterModule registers a new host module having the provided name (moduleName).
// After a module is added, the registered module cannot be updated and given
// new functions.
func (m *HostModuleRegistry) RegisterModule(moduleName string, funcs []*HostModuleFunction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	builder := m.runtime.NewHostModuleBuilder(moduleName)
	for _, fn := range funcs {
		builder.NewFunctionBuilder().
			WithGoModuleFunction(fn.GoFunc, fn.Params, fn.Results).
			WithName(fn.Name).
			Export(fn.Name)
	}
	mod, err := builder.Instantiate(m.ctx)
	if err != nil {
		return err
	}
	m.modules = append(m.modules, mod)
	return nil
}

// Close invokes the [api.Closer.Close] function of the registered host modules.
// Returns an error if one of them failed to close.
func (m *HostModuleRegistry) Close() error {
	for _, mod := range m.modules {
		if mod == nil {
			continue
		}
		if err := mod.Close(m.ctx); err != nil {
			return err
		}
	}
	return nil
}

// Iter provides the iterator over the registered modules.
func (m *HostModuleRegistry) Iter() iter.Seq[api.Module] {
	return func(yield func(api.Module) bool) {
		for _, m := range m.modules {
			if !yield(m) {
				break
			}
		}
	}
}

func (m *HostModuleRegistry) SetRuntime(r wazero.Runtime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runtime = r
}
