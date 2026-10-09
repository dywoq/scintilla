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
	IncludeDirs   []string    `toml:"include_dirs"`
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
	if err := os.Mkdir(filepath.Join(p.Dir, p.Conf.ObjectsDir), os.ModeAppend|os.ModePerm); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	baseArgs := p.baseCompileArgs()
	for _, src := range p.Conf.Sources {
		objFilename, _ := strings.CutSuffix(src, ".cxx")
		objFilename += ".obj"
		objPath := filepath.Join(p.Conf.ObjectsDir, objFilename)
		finalArgs := slices.Concat(baseArgs, []string{
			"-c",
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

// Link invokes [Package.Compile] and retrieves all object files in the
// objects directory. Executes `clang++` to link a program. Returns an error if
// the current config type (p.Conf.Type) is not [PackageTypeProgram].
func (p *Package) Link() error {
	if err := p.Compile(); err != nil {
		return err
	}
	if p.Conf.Type != PackageTypeProgram {
		return fmt.Errorf("cannot link non-program package")
	}

	objects := p.objectFiles()
	if objects == nil {
		return fmt.Errorf("objects are not found in the %q directory", p.Conf.ObjectsDir)
	}
	if _, err := os.Lstat(p.Conf.TargetFile); os.IsExist(err) {
		os.Remove(p.Conf.TargetFile)
	}
	args := slices.Concat(p.baseCompileArgs(), []string{
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

// Archive invokes [Package.Compile] and retrieves all object files in the
// objects directory. Executes `llvm-ar` to link a program. Returns an error if
// the current config type (p.Conf.Type) is not [PackageTypeProgram]
func (p *Package) Archive() error {
	if err := p.Compile(); err != nil {
		return err
	}
	if p.Conf.Type != PackageTypeLib {
		return fmt.Errorf("cannot archive non-library package")
	}
	objects := p.objectFiles()
	if objects == nil {
		return fmt.Errorf("objects are not found in the %q directory", p.Conf.ObjectsDir)
	}
	if _, err := os.Lstat(p.Conf.TargetFile); os.IsExist(err) {
		os.Remove(p.Conf.TargetFile)
	}
	args := slices.Concat([]string{
		"rcs",
		p.Conf.TargetFile,
	}, objects)
	cmd := exec.Command(
		"llvm-ar",
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

// GenerateCompilationDatabase is similar to [Package.Compile], but it inserts
// `bear --` before `clang++` to generate a compilation database that works
// with clangd.
func (p *Package) GenerateCompilationDatabase() error {
	objects := p.objectFiles()
	if objects == nil {
		return fmt.Errorf("objects are not found in the %q directory", p.Conf.ObjectsDir)
	}
	baseArgs := p.baseCompileArgs()
	for _, src := range p.Conf.Sources {
		objFilename, _ := strings.CutSuffix(src, ".cxx")
		objFilename += ".obj"
		objPath := filepath.Join(p.Conf.ObjectsDir, objFilename)
		finalArgs := slices.Concat([]string{"--", "clang++"}, baseArgs, []string{
			"-c",
			"-o",
			objPath,
			src,
		})
		cmd := exec.Command(
			"bear",
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

func (p *Package) objectFiles() []string {
	// Since the working directory before cmd.CombinedOutput is not p.Dir, we need to use
	// p.Dir directory in the glob pattern temporarily. After finding object files, we trim
	// the prefix.
	objects, _ := filepath.Glob(filepath.Join(p.Dir, p.Conf.ObjectsDir, "*.obj"))
	if objects != nil {
		for idx, obj := range objects {
			before, _ := strings.CutPrefix(obj, p.Dir+"/")
			objects[idx] = before
		}
	}
	return objects
}

func (p *Package) baseCompileArgs() []string {
	baseArgs := slices.Concat([]string{
		"--target=wasm32-unknown-unknown",
		"-nostdlib",
	}, p.Conf.CompilerFlags)
	for _, inc := range p.Conf.IncludeDirs {
		baseArgs = append(baseArgs, fmt.Sprintf("-I%s", inc))
	}
	return baseArgs
}
