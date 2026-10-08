// Copyright 2026 Bellus Development Team - All Rights Reserved

package scintilla

import (
	"context"
	"fmt"
	"iter"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type ModuleFunction struct {
	Name    string
	GoFunc  api.GoModuleFunc
	Params  []api.ValueType
	Results []api.ValueType
}

// ModuleRegistry is a centralized storage, which registers provided host modules
// to [wazero.Runtime]. It is used by [Script] instances. It is thread safe.
type ModuleRegistry struct {
	modules []api.Module
	mu      sync.Mutex
	runtime wazero.Runtime
	ctx     context.Context
}

func NewModuleFunction(name string, goFunc api.GoModuleFunc, params []api.ValueType, results []api.ValueType) *ModuleFunction {
	return &ModuleFunction{
		Name:    name,
		GoFunc:  goFunc,
		Params:  params,
		Results: results,
	}
}

// NewModuleRegistry pre-allocates the underlying modules slice with the capacity as 4.
func NewModuleRegistry(ctx context.Context) *ModuleRegistry {
	return &ModuleRegistry{
		ctx:     ctx,
		modules: make([]api.Module, 0, 4),
	}
}

// RegisterModule registers a new module having the provided name (moduleName).
// After a module is added, the registered module cannot be updated and given
// new functions.
func (m *ModuleRegistry) RegisterModule(moduleName string, funcs []*ModuleFunction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	builder := m.runtime.NewHostModuleBuilder(moduleName)
	for _, fn := range funcs {
		fmt.Println(moduleName, fn.Name)
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

// Close invokes the [api.Closer.Close] function of the registered modules.
// Returns an error if one of them failed to close.
func (m *ModuleRegistry) Close() error {
	for _, mod := range m.modules {
		if err := mod.Close(m.ctx); err != nil {
			return err
		}
	}
	return nil
}

// Iter provides the iterator over the registered modules.
func (m *ModuleRegistry) Iter() iter.Seq[api.Module] {
	return func(yield func(api.Module) bool) {
		for _, m := range m.modules {
			if !yield(m) {
				break
			}
		}
	}
}

func (m *ModuleRegistry) SetRuntime(r wazero.Runtime) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runtime = r
}
