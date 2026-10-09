// Copyright 2026 Bellus Development Team - All Rights Reserved

package scintilla

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	Dependencies  []string    `toml:"dependencies"`
}

type Package struct {
	Dir          string
	Conf         *PackageConf `toml:"config"`
	ConfMetadata toml.MetaData
}

type PackageEnv struct {
	MainPackage        *Package
	DependencyPackages []*Package
	TempDir            string
}

func NewPackage(dir string) *Package {
	return &Package{Conf: new(PackageConf), Dir: dir}
}

func NewPackageEnv(mainPackageDir string, tempDir string) *PackageEnv {
	return &PackageEnv{
		MainPackage:        NewPackage(mainPackageDir),
		DependencyPackages: []*Package{},
		TempDir:            tempDir,
	}
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
	metadata, err := toml.DecodeFile("scintilla.toml", &conf)
	if err != nil {
		return err
	}
	p.ConfMetadata = metadata
	p.Conf = conf
	return nil
}

// Valid checks if scintilla.toml file exists.
func (p *Package) Exists() bool {
	_, configErr := os.Lstat("scintilla.toml")
	if configErr != nil && os.IsNotExist(configErr) {
		return false
	}
	return true
}

// CreateGitignore adds the .gitignore file to the current directory.
// If .gitignore already exists, it is truncated.
//
// Returns an error if it failed to create a .gitignore file in the
// package directory.
func (p *Package) CreateGitignore() error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", p.Conf.ObjectsDir)
	b.WriteString("compile_commands.json\n")
	b.WriteString(p.Conf.TargetFile)
	b.WriteRune('\n')
	f, err := os.Create(".gitignore")
	if err != nil {
		return err
	}
	defer f.Close()
	f.WriteString(b.String())
	return nil
}

// Compile invokes the clang++ executable and compiles the source files,
// specified in [PackageConf]. It puts them into the package's objects
// directory (p.Conf.ObjectsDir), which is created if it does not exist.
//
// Returns an error if it failed to create an objects directory or
// invoke the `clang++` executable (prints its combined output into the
// console on error).
func (p *Package) Compile() error {
	if err := os.Mkdir(p.Conf.ObjectsDir, os.ModeAppend|os.ModePerm); os.IsExist(err) {
		os.RemoveAll(p.Conf.ObjectsDir)
		os.Mkdir(p.Conf.ObjectsDir, os.ModeAppend|os.ModePerm)
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
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Print(string(output))
			return err
		}
	}
	return nil
}

// Link invokes [Package.Compile] and retrieves all object files in the
// objects directory. Executes `clang++` to link a program.
//
// Returns an error if the current config type (p.Conf.Type) is not
// [PackageTypeProgram], object files are not found or invoking `clang++`
// failed (prints its combined output into the console on error).
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Print(string(output))
		return err
	}
	return nil
}

// Archive invokes [Package.Compile] and retrieves all object files in the
// objects directory. Executes `llvm-ar` to link a program.
//
// Returns an error if the current config type (p.Conf.Type) is not
// [PackageTypeLib], object files are not found or invoking `llvm-ar`
// failed (prints its combined output into the console on error).
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
//
// Returns an error if invoking `bear` failed (prints its combined output into
// the console on error).
func (p *Package) GenerateCompilationDatabase() error {
	if err := os.Mkdir(p.Conf.ObjectsDir, os.ModeAppend|os.ModePerm); os.IsExist(err) {
		os.RemoveAll(p.Conf.ObjectsDir)
		os.Mkdir(p.Conf.ObjectsDir, os.ModeAppend|os.ModePerm)
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
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Print(string(output))
			return err
		}
	}
	return nil
}

// Pack retrieves the configuration type. If it is [PackageTypeProgram],
// it compiles and links the program before turning it into archive, which has
// scintilla.toml. If it is [PackageTypeLib], Pack compiles object files,
// calls [Package.Archive] and writes all include directories to the archive
// in the original layout.
func (p *Package) Pack() error {
	switch p.Conf.Type {
	case PackageTypeProgram:
		return p.packProgram()
	case PackageTypeLib:
		return p.packLibrary()
	}
	return fmt.Errorf("unknown package type %q", p.Conf.Type)
}

func (p *Package) packProgram() error {
	if err := p.Compile(); err != nil {
		return err
	}
	if err := p.Link(); err != nil {
		return err
	}

	f, err := os.Create(fmt.Sprintf("%s.zip", p.Dir))
	if err != nil {
		return err
	}
	defer f.Close()

	writer := zip.NewWriter(f)
	if err := p.packFile(writer, p.Conf.TargetFile, p.Conf.TargetFile); err != nil {
		writer.Close()
		return err
	}

	if err := p.packFile(writer, "scintilla.toml", "scintilla.toml"); err != nil {
		writer.Close()
		return err
	}

	return writer.Close()
}

func (p *Package) packLibrary() error {
	if err := p.Compile(); err != nil {
		return err
	}
	if err := p.Archive(); err != nil {
		return err
	}

	f, err := os.Create(fmt.Sprintf("%s.zip", p.Dir))
	if err != nil {
		return err
	}
	defer f.Close()

	writer := zip.NewWriter(f)
	if err := p.packFile(writer, p.Conf.TargetFile, p.Conf.TargetFile); err != nil {
		writer.Close()
		return err
	}

	if err := p.packFile(writer, "scintilla.toml", "scintilla.toml"); err != nil {
		writer.Close()
		return err
	}

	for _, inc := range p.Conf.IncludeDirs {
		if err := p.packDirectory(writer, inc); err != nil {
			writer.Close()
			return err
		}

	}

	return writer.Close()
}

func (p *Package) packFile(writer *zip.Writer, physicalFilepath string, zipFilepath string) error {
	w, err := writer.Create(zipFilepath)
	if err != nil {
		return err
	}

	targetFile, err := os.Open(physicalFilepath)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(w, targetFile); err != nil {
		return err
	}

	return nil
}

func (p *Package) packDirectory(writer *zip.Writer, sourceDir string) error {
	sourceDir = filepath.Clean(sourceDir)
	parentDir := filepath.Dir(sourceDir)
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(parentDir, path)
		if err != nil {
			return err
		}

		zipPath := filepath.ToSlash(relPath)

		info, err := entry.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = zipPath

		if entry.IsDir() {
			header.Name += "/"
			_, err := writer.CreateHeader(header)
			return err
		}

		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file type: %s", path)
		}

		header.Method = zip.Deflate

		w, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		_, copyErr := io.Copy(w, file)
		closeErr := file.Close()

		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func (p *Package) objectFiles() []string {
	objects, _ := filepath.Glob(filepath.Join(p.Conf.ObjectsDir, "*.obj"))
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

// UnpackDependencies iterates over the main package's dependencies archive files
// and unpacks them using [UnpackPackage]. Creates the specified temp directory
// if it does not exist. If a current unpacking dependency is a library,
// the function appends its include directories to the main package.
func (p *PackageEnv) UnpackDependencies() error {
	if p.MainPackage.Conf == nil {
		return errors.New("main package config is to be parsed")
	}
	if err := p.createTmpDir(); err != nil {
		return err
	}
	for _, file := range p.MainPackage.Conf.Dependencies {
		dest := filepath.Join(p.tmpDependenciesPath(), file)
		d, err := UnpackPackage(file, dest)
		if err != nil {
			return err
		}
		if err := d.ParseConfig(); err != nil {
			return err
		}
		if d.Conf.Type == PackageTypeLib {
			for _, inc := range d.Conf.IncludeDirs {
				str := filepath.Join(dest, inc)
				p.MainPackage.Conf.IncludeDirs = append(p.MainPackage.Conf.IncludeDirs, str)
			}
		}
		p.DependencyPackages = append(p.DependencyPackages, d)
	}
	return nil
}

func (p *PackageEnv) createTmpDir() error {
	// This function is supposed to create temporary directory and its entries
	// if it does not exist
	if _, err := os.Lstat(p.TempDir); os.IsNotExist(err) {
		if err := os.Mkdir(p.TempDir, os.ModeAppend|os.ModePerm); err != nil {
			return err
		}
		if err := os.Mkdir(p.tmpDependenciesPath(), os.ModeAppend|os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func (p *PackageEnv) tmpDependenciesPath() string {
	return filepath.Join(p.TempDir, "dependencies")
}

// UnpackPackage opens the ZIP archive file at archivePath and writes its contents to destFolder.
// If destFolder does not exist, it is created in the process's working directory. UnpackPackage
// parses scintilla.toml, which is at the root of the archive file. If it is a library,
// UnpackPackage unpacks include directories in the original tree layout.
//
// Returns an error if UnpackPackage failed to create destination folder, open and parse
// scintilla.toml and copy include directories to the destFolder.
func UnpackPackage(archivePath string, destFolder string) (*Package, error) {
	zipR, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer zipR.Close()

	if _, err := os.Lstat(destFolder); os.IsNotExist(err) {
		if err := os.Mkdir(destFolder, os.ModePerm|os.ModeAppend); err != nil {
			return nil, err
		}
	}

	var p *Package
	parsedPackageConf := false
	for _, file := range zipR.File {
		if file.Name == "scintilla.toml" {
			configFilepath := filepath.Join(destFolder, "scintilla.toml")
			r, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			f, err := os.Create(configFilepath)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			if _, err := io.Copy(f, r); err != nil {
				return nil, err
			}

			p = NewPackage(destFolder)
			if err := p.ParseConfig(); err != nil {
				return nil, err
			}
			parsedPackageConf = true
			continue
		}
	}
	if !parsedPackageConf {
		return nil, fmt.Errorf("config scintilla.toml is not found in %q archive", archivePath)
	}

	if p.Conf.Type == PackageTypeLib {
		for _, file := range zipR.File {
			if file.FileInfo().IsDir() && slices.Contains(p.Conf.IncludeDirs, file.Name) {
				if err := unpackPackageDirectory(zipR, file.Name, filepath.Join(destFolder, file.Name)); err != nil {
					return nil, err
				}
			}
		}
	}

	for _, file := range zipR.File {
		if parsedPackageConf && p.Conf.TargetFile == file.Name {
			targetPath := filepath.Join(destFolder, p.Conf.TargetFile)
			r, err := file.Open()
			if err != nil {
				return nil, err
			}

			f, err := os.Create(targetPath)
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(f, r); err != nil {
				return nil, err
			}
		}
	}

	return p, nil
}

func unpackPackageDirectory(r *zip.ReadCloser, archiveDir, dest string) error {
	archiveDir = strings.Trim(archiveDir, "/")
	if archiveDir == "" || !filepath.IsLocal(filepath.FromSlash(archiveDir)) {
		return fmt.Errorf("invalid archive directory: %q", archiveDir)
	}

	root, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("resolve destination: %w", err)
	}

	for _, f := range r.File {
		name := f.Name

		if strings.Contains(name, `\`) {
			return fmt.Errorf("invalid ZIP entry: %q", name)
		}

		name = strings.TrimSuffix(name, "/")

		if !strings.HasPrefix(name, archiveDir+"/") {
			continue
		}

		rel := strings.TrimPrefix(name, archiveDir+"/")
		if rel == "" || !filepath.IsLocal(filepath.FromSlash(rel)) {
			return fmt.Errorf("invalid ZIP entry path: %q", f.Name)
		}

		target := filepath.Join(root, filepath.FromSlash(rel))

		mode := f.Mode()
		if mode&os.ModeSymlink != 0 ||
			(!mode.IsRegular() && !mode.IsDir()) {
			return fmt.Errorf("unsupported ZIP entry type: %q", f.Name)
		}

		if mode.IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("create directory %q: %w", target, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("create parent directory: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open ZIP entry %q: %w", f.Name, err)
		}

		out, err := os.OpenFile(
			target,
			os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
			mode.Perm()&0755,
		)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create file %q: %w", target, err)
		}

		_, copyErr := io.Copy(out, rc)
		outErr := out.Close()
		rcErr := rc.Close()

		if copyErr != nil {
			return fmt.Errorf("extract %q: %w", f.Name, copyErr)
		}
		if outErr != nil {
			return fmt.Errorf("close output %q: %w", target, outErr)
		}
		if rcErr != nil {
			return fmt.Errorf("close ZIP entry %q: %w", f.Name, rcErr)
		}
	}

	return nil
}
