// Copyright 2026 Bellus Development Team - All Rights Reserved

package scintilla

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// Script contains the current running script's information.
type Script struct {
	filepath string
	filedata []byte

	runtime wazero.Runtime

	apiMod api.Module

	mu  sync.Mutex
	ctx context.Context

	initialized         atomic.Bool
	loadedModApi        atomic.Bool
	isRunningFuncCached atomic.Bool

	registry *HostModuleRegistry

	cachedRunningFuncApi api.Function
	cachedFuncName string
}

var (
	ErrNotInitialized error = errors.New("not initialize")
)

// NewScript constructs a new [Script] instance. It does not initializes
// [Script] resources.
func NewScript(ctx context.Context, filepath string) *Script {
	return &Script{filepath: filepath, ctx: ctx}
}

// Run caches the underlying [api.Module] and running function of the script lazily.
// However, caching [api.Function] is skipped if you run the same function twice.
// Invokes the [api.Function.Call] function and returns its error.
//
// Returns [ErrNotInitialized] if the function [Script.Init] has not
// run successfully.
//
// Returns an error if caching [api.Module] and [api.Function] failed.
func (s *Script) Run(funcName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loadedModApi.Load() {
		if err := s.loadModApi(); err != nil {
			return err
		}
	}
	if !s.initialized.Load() {
		return ErrNotInitialized
	}
	err := s.cacheRunningFunc(funcName)
	if err != nil {
		return err
	}
	_, err = s.cachedRunningFuncApi.Call(s.ctx)
	return err
}

// Close closes the script resources of the underlying [api.Module] and [wazero.Runtime].
// Returns an error these functions returned.
func (s *Script) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.close()
}

// Init initializes the underlying [wazero.Runtime]. Init shall be called
// before modules are added using [HostModuleRegistry].
func (s *Script) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.init()
}

// SetModuleRegistry sets the provided [HostModuleRegistry]. SetModuleRegistry
// shall be called before modules are added using [HostModuleRegistry].
func (s *Script) SetModuleRegistry(m *HostModuleRegistry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registry = m
	s.registry.SetRuntime(s.runtime)
}

// Runtime provides direct access to the underlying [wazero.Runtime] instance.
func (s *Script) Runtime() wazero.Runtime {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runtime
}

func (s *Script) close() error {
	if !s.initialized.Load() {
		return nil
	}
	err := s.apiMod.Close(s.ctx)
	if err != nil {
		return err
	}

	err = s.runtime.Close(s.ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *Script) init() error {
	filedata, err := os.ReadFile(s.filepath)
	if err != nil {
		return err
	}
	s.filedata = filedata
	s.runtime = wazero.NewRuntime(s.ctx)
	s.initialized.Store(true)
	return nil
}

func (s *Script) loadModApi() error {
	config := wazero.NewModuleConfig().WithStartFunctions()
	mod, err := s.runtime.InstantiateWithConfig(s.ctx, s.filedata, config)
	if err != nil {
		return err
	}
	s.apiMod = mod
	s.loadedModApi.Store(true)
	return nil
}

func (s *Script) cacheRunningFunc(funcName string) error {
	// To cache the running function and prevent heavy operations such as loading
	// a function into memory constantly, we need to track the following:
	//
	// 		- current running function's name
	// 		- whether it has been cached already or not
	// 		- store the instance of the api.Function interface once we cache it
	//
	// To update the instance, we check if funcName and s.cachedFuncName
	// match. If they do not, we update it.
	if s.cachedFuncName == funcName {
		return nil
	}
	funcApi := s.apiMod.ExportedFunction(funcName)
	if funcApi == nil {
		return fmt.Errorf("an exported function %q is not found", funcName)
	}
	s.cachedRunningFuncApi = funcApi
	s.cachedFuncName = funcName
	return nil
}
