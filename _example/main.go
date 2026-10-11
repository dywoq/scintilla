package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dywoq/scintilla"
	"github.com/tetratelabs/wazero/api"
)

func main() {
	// Compile and link the "program" package, setting the build mode to release.
	// Along with that, we parse its configuration file, build and unpack its dependencies.
	// The order matters, because PackageEnv depends on the package's configuration
	p := scintilla.NewPackage("./program")
	p.BuildMode = scintilla.PackageBuildModeRelease
	if err := p.ParseConfig(); err != nil {
		panic(err)
	}

	env := scintilla.NewPackageEnv(p.Dir)
	env.MainPackage = p

	if err := env.BuildDependencies(); err != nil {
		panic(err)
	}
	if err := env.UnpackDependencies(); err != nil {
		panic(err)
	}

	if err := p.Link(); err != nil {
		panic(err)
	}

	// Initialize the script
	ctx := context.Background()
	script := scintilla.NewScript(ctx, filepath.Join(p.Dir, p.Conf.TargetFile))
	if err := script.Init(); err != nil {
		panic(err)
	}
	defer script.Close()

	// Provide the script with functionality to print a number
	moduleRegistry := scintilla.NewHostModuleRegistry(ctx)
	script.SetModuleRegistry(moduleRegistry)
	moduleRegistry.RegisterModule("host", []*scintilla.HostModuleFunction{
		scintilla.NewHostModuleFunction("print_number", func(ctx context.Context, mod api.Module, stack []uint64) {
			num := api.DecodeI32(stack[0])
			fmt.Printf("%d\n", num)
		}, []api.ValueType{api.ValueTypeI32}, []api.ValueType{}),
		scintilla.NewHostModuleFunction("print_string", func(ctx context.Context, mod api.Module, stack []uint64) {
			offset := api.DecodeU32(stack[0])
			length := api.DecodeU32(stack[1])
			mem, ok := mod.Memory().Read(offset, length)
			if !ok {
				fmt.Printf("Failed to read memory (offset: 0x%X, length: 0x%X)\n", offset, length)
				return
			}
			fmt.Printf("%v", string(mem))
		}, []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{}),
	})
	defer moduleRegistry.Close()

	// Run the _start function
	if err := script.Run("_start"); err != nil {
		panic(err)
	}
}
