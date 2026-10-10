// Copyright 2026 Bellus Development Team - All Rights Reserved

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/bellus-team/scintilla"
	"github.com/spf13/cobra"
)

func Init() *cobra.Command {
	init := &cobra.Command{
		Use:   "init [package_name]",
		Args:  cobra.ExactArgs(1),
		Short: "Initialize your package",
		RunE: func(cmd *cobra.Command, args []string) error {
			packageName := args[0]
			tType, _ := cmd.Flags().GetString("type")
			packageType := scintilla.PackageType(tType)
			if !packageType.Valid() {
				return fmt.Errorf("unknown package type: %q", tType)
			}

			if err := os.Mkdir(packageName, os.ModeAppend|os.ModePerm); err != nil {
				return fmt.Errorf("creating %q directory failed: %v", packageName, err)
			}

			conf := &scintilla.PackageConf{
				Type:          packageType,
				ObjectsDir:    ".objs",
				CompilerFlags: []string{},
				LinkerFlags:   []string{},
				IncludeDirs:   []string{},
				LibraryDirs:   []string{},
				Dependencies:  []string{},
			}

			switch packageType {
			case scintilla.PackageTypeLib:
				conf.TargetFile = fmt.Sprintf("lib%s.a", packageName)
				conf.PackedFile = fmt.Sprintf("lib%s.zip", packageName)
				if err := initLibrary(packageName, conf); err != nil {
					return err
				}
			case scintilla.PackageTypeProgram:
				conf.TargetFile = fmt.Sprintf("%s.wasm", packageName)
				conf.PackedFile = fmt.Sprintf("%s.zip", packageName)
				if err := initProgram(packageName, conf); err != nil {
					return err
				}
			}

			f, err := os.Create(filepath.Join(packageName, "scintilla.toml"))
			if err != nil {
				return fmt.Errorf("creating scintilla.toml failed: %v", err)
			}
			if err := toml.NewEncoder(f).Encode(&conf); err != nil {
				return err
			}
			f.Close()

			return nil
		},
	}
	init.Flags().String("type", "program", "package's type")
	return init
}

func initProgram(packageName string, conf *scintilla.PackageConf) error {
	mainFilename := "main.cxx"
	p := scintilla.NewPackage(packageName)
	p.Conf = conf
	p.Conf.Sources = []string{mainFilename}
	var mainContent strings.Builder
	mainContent.WriteString("extern \"C\" void _start() {\n\t")
	mainContent.WriteString("\n}")
	mainFile, err := os.Create(filepath.Join(packageName, mainFilename))
	if err != nil {
		return err
	}
	io.WriteString(mainFile, mainContent.String())
	mainFile.Close()
	return p.CreateGitignore()
}

func initLibrary(packageName string, conf *scintilla.PackageConf) error {
	fooFilename := "foo.cxx"
	p := scintilla.NewPackage(packageName)
	p.Conf = conf
	p.Conf.Sources = []string{fooFilename}
	var fooContent strings.Builder
	fooContent.WriteString("void foo() {\n\t")
	fooContent.WriteString("\n}")
	fooFile, err := os.Create(filepath.Join(packageName, fooFilename))
	if err != nil {
		return err
	}
	io.WriteString(fooFile, fooContent.String())
	fooFile.Close()
	return p.CreateGitignore()
}
