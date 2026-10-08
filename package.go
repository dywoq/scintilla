// Copyright 2026 Bellus Development Team - All Rights Reserved

package scintilla

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// PackageType identifies the type of a Scintilla package.
type PackageType string

// PackageConf contains the TOML configuration.
type PackageConf struct {
	TargetFile    string      `toml:"target_file"`
	Type          PackageType `toml:"type"`
	Sources       []string    `toml:"sources"`
	ObjectsDir    string      `toml:"objects_dir"`
	CompilerFlags []string    `toml:"compiler_flags"`
	LinkerFlags   []string    `toml:"linker_flags"`
}

type Package struct {
	Dir          string
	Conf         *PackageConf `toml:"config"`
	ConfMetadata toml.MetaData
}

func NewPackage(dir string) *Package {
	return &Package{Conf: new(PackageConf), Dir: dir}
}

const (
	PackageTypeLib     PackageType = "lib"
	PackageTypeProgram PackageType = "program"
)

// Valid checks if p equals to all constants having the [PackageType] type.
func (p PackageType) Valid() bool {
	switch p {
	case PackageTypeLib, PackageTypeProgram:
		return true
	default:
		return false
	}
}

// ParseConfig decodes the p.dir/scintilla.toml configuration file using [toml.DecodeFile],
// and writes the parsed [PackageConf] to p.Conf along with its metadata.
func (p *Package) ParseConfig() error {
	var conf *PackageConf
	metadata, err := toml.DecodeFile(filepath.Join(p.Dir, "scintilla.toml"), &conf)
	if err != nil {
		return err
	}
	p.ConfMetadata = metadata
	p.Conf = conf
	return nil
}

// Valid checks if p.Dir and p.Dir/scintilla.toml file exist.
func (p *Package) Exists() bool {
	_, dirErr := os.Lstat(p.Dir)
	_, configErr := os.Lstat(filepath.Join(p.Dir, "scintilla.toml"))
	if dirErr != nil && os.IsNotExist(dirErr) {
		return false
	}
	if configErr != nil && os.IsNotExist(configErr) {
		return false
	}
	return true
}

// CreateGitignore adds the .gitignore file to the package directory p.Dir.
// If .gitignore already exists, it is truncated.
func (p *Package) CreateGitignore() error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", p.Conf.ObjectsDir)
	b.WriteString("compile_commands.json\n")
	b.WriteString(p.Conf.TargetFile)
	b.WriteRune('\n')
	f, err := os.Create(filepath.Join(p.Dir, ".gitignore"))
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(b.String())
	return nil
}

// Compile invokes the clang++ executable and compiles the source files,
// specified in [PackageConf].
//
// It puts them into the package's objects directory (p.Conf.ObjectsDir),
// which is created if it does not exist.
func (p *Package) Compile() error {
	baseArgs := slices.Concat([]string{
		"--target=wasm32-unknown-unknown",
		"-nostdlib",
		"-c",
	}, p.Conf.CompilerFlags)
	if err := os.Mkdir(filepath.Join(p.Dir, p.Conf.ObjectsDir), os.ModeAppend|os.ModePerm); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	for _, src := range p.Conf.Sources {
		objFilename := strings.TrimRight(src, ".cxx") + ".obj"
		objPath := filepath.Join(p.Conf.ObjectsDir, objFilename)
		finalArgs := slices.Concat(baseArgs, []string{
			"-o",
			objPath,
			src,
		})
		cmd := exec.Command(
			"clang++",
			finalArgs...,
		)
		cmd.Dir = p.Dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Print(string(output))
			return err
		}
	}
	return nil
}

func (p *Package) Link() error {
	if err := p.Compile(); err != nil {
		return err
	}
	// Since the working directory before cmd.CombinedOutput is not p.Dir, we need to use
	// p.Dir directory in the glob pattern temporarily. After finding object files, we trim
	// the prefix.
	objects, _ := filepath.Glob(filepath.Join(p.Dir, p.Conf.ObjectsDir, "*.obj"))
	if objects == nil {
		return fmt.Errorf("objects are not found in the %q directory", p.Conf.ObjectsDir)
	}
	for idx, obj := range objects {
		objects[idx] = strings.TrimLeft(obj, p.Dir+"/")
	}
	args := slices.Concat([]string{
		"--target=wasm32-unknown-unknown",
		"-nostdlib",
		"-Wl,--export-all",
		"-Wl,--no-entry",
	}, p.Conf.LinkerFlags, objects, []string{"-o", p.Conf.TargetFile})
	cmd := exec.Command(
		"clang++",
		args...,
	)
	cmd.Dir = p.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Print(string(output))
		return err
	}
	return nil
}
