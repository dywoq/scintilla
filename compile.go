// Copyright 2026 Bellus Development Team - All Rights Reserved

package scintilla

import (
	"fmt"
	"os/exec"
)

type Config struct {
	// OutputFile specifies the output WASM file destination. Notice that this file
	// is put in the tinygo's working directory, which is the module directory
	// you provide to [CompileWithConfig].
	OutputFile string

	// PrintOutput decides whether to print tinygo's output if its exit status is not 0.
	PrintOutput bool
}

// Compile invokes the `tinygo` executable, providing it with the flags required
// for a Scintilla module. The working directory is moduleDir.
func CompileWithConfig(moduleDir string, c *Config) error {
	cmd := exec.Command(
		"tinygo",
		"build",
		"-o",
		c.OutputFile,
		"-gc=none",
		"-scheduler=none",
		"-target=wasm-unknown",
		".",
	)
	cmd.Dir = moduleDir
	combinedOutput, err := cmd.CombinedOutput()
	if err != nil {
		if c.PrintOutput {
			fmt.Print(string(combinedOutput))
		}
		return fmt.Errorf("running tinygo failed: %v", err)
	}
	return nil
}

// Compile wraps around [CompileWithConfig]. By default, the output file is "<moduleDir>.wasm"
// and [CompileWithConfig] prints tinygo's output.
func Compile(moduleDir string) error {
	return CompileWithConfig(moduleDir, &Config{
		OutputFile:  fmt.Sprintf("%s.wasm", moduleDir),
		PrintOutput: true,
	})
}
