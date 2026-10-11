// Copyright 2026 dywoq - Apache License 2.0
// https://github.com/dywoq/scintilla

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

// PackageBuildMode identifies the build mode of a package. Depending on the chosen build mode,
// a set of corresponding macros and flags are activated, which are described in the documentation
// of PackageBuildMode's constants.
type PackageBuildMode string

// PackageConf contains the TOML configuration.
type PackageConf struct {
	TargetFile         string      `toml:"target_file"`
	PackedFile         string      `toml:"packed_file"`
	Type               PackageType `toml:"type"`
	Sources            []string    `toml:"sources"`
	ObjectsDir         string      `toml:"objects_dir"`
	CompilerFlags      []string    `toml:"compiler_flags"`
	LinkerFlags        []string    `toml:"linker_flags"`
	IncludeDirs        []string    `toml:"include_dirs"`
	LibraryDirs        []string    `toml:"library_dirs"`
	DependencyZipFiles []string    `toml:"dependency_zip_files"`
	DependencyPaths    []string    `toml:"dependency_paths"`
}

type Package struct {
	Dir          string
	Conf         *PackageConf `toml:"config"`
	ConfMetadata toml.MetaData
	BuildMode    PackageBuildMode
}

type PackageEnv struct {
	MainPackage        *Package
	DependencyPackages []*Package
}

// PackageErrorSrc identifies the origin of an error.
type PackageErrorSrc uint8

type PackageError struct {
	Err error
	Src PackageErrorSrc
}

func NewPackage(dir string) *Package {
	return &Package{Conf: new(PackageConf), Dir: dir}
}

func NewPackageEnv(mainPackageDir string) *PackageEnv {
	return &PackageEnv{
		MainPackage:        NewPackage(mainPackageDir),
		DependencyPackages: []*Package{},
	}
}

const (
	PackageTypeLib     PackageType = "lib"
	PackageTypeProgram PackageType = "program"
)

const (
	// PackageBuildModeRelease is a build mode where a package's build is optimized and
	// has no debugging symbols. The following macros are defined:
	//
	// 	RELEASE
	//
	// The following flags are applied to Clang compiler:
	//
	// 	-O2
	PackageBuildModeRelease PackageBuildMode = "release"

	// PackageBuildModeRelease is a build mode where a package's build is non-optimized and
	// has debugging symbols. The following macros are defined:
	//
	// 	DEBUG
	PackageBuildModeDebug PackageBuildMode = "debug"
)

const (
	PackageErrorSrcCompile PackageErrorSrc = iota
	PackageErrorSrcLink
	PackageErrorSrcCompilationDatabase
	PackageErrorSrcArchive
	PackageErrorSrcPack
	PackageErrorSrcUnpack
	PackageErrorSrcBuildDependencies
)

var (
	ErrInvalidPackageBuildMode = errors.New("invalid package's build mode")
	ErrUnknownPackageType      = errors.New("unknown package type")
	ErrPackageConfNotParsed    = errors.New("package config is not parsed")
)

// Valid checks if p equals to any of constants having the [PackageType] type.
func (p PackageType) Valid() bool {
	switch p {
	case PackageTypeLib, PackageTypeProgram:
		return true
	default:
		return false
	}
}

// Valid checks if p equals to any of constants having the [PackageBuildMode] type.
func (p PackageBuildMode) Valid() bool {
	switch p {
	case PackageBuildModeRelease, PackageBuildModeDebug:
		return true
	default:
		return false
	}
}

func (p PackageErrorSrc) String() string {
	switch p {
	case PackageErrorSrcCompile:
		return "Compile"
	case PackageErrorSrcLink:
		return "Link"
	case PackageErrorSrcCompilationDatabase:
		return "CompilationDatabase"
	case PackageErrorSrcArchive:
		return "Archive"
	case PackageErrorSrcPack:
		return "Pack"
	case PackageErrorSrcUnpack:
		return "Unpack"
	case PackageErrorSrcBuildDependencies:
		return "BuildDependencies"
	}
	return "unknown"
}

func (p *PackageError) Error() string {
	return fmt.Sprintf("%v { source: %s }", p.Err, p.Src)
}

// ParseConfig decodes the p.Dir/scintilla.toml configuration file using [toml.DecodeFile],
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

// Valid checks if scintilla.toml file exists.
func (p *Package) Exists() bool {
	_, configErr := os.Lstat(filepath.Join(p.Dir, "scintilla.toml"))
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
	b.WriteString(".scintilla\n")
	b.WriteString("*.zip")
	f, err := os.Create(filepath.Join(p.Dir, ".gitignore"))
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
//
// Returns [ErrInvalidPackageBuildMode] if [PackageBuildMode.Valid] returns
// false.
func (p *Package) Compile() error {
	if !p.BuildMode.Valid() {
		return &PackageError{
			Err: ErrInvalidPackageBuildMode,
			Src: PackageErrorSrcCompile,
		}
	}
	if err := p.createObjsDir(); err != nil {
		return &PackageError{
			Src: PackageErrorSrcCompile,
			Err: fmt.Errorf("failed to create objects directory: %v", err),
		}
	}
	baseArgs := p.baseCompileArgs(true)
	for _, src := range p.Conf.Sources {
		finalSrc, objPath := p.convertSrcToObj(src, true)
		finalArgs := slices.Concat(baseArgs, []string{
			"-c",
			"-o",
			objPath,
			finalSrc,
		})
		cmd := exec.Command(
			"clang++",
			finalArgs...,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Print(string(output))
			return &PackageError{
				Err: err,
				Src: PackageErrorSrcCompile,
			}
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
		return &PackageError{
			Err: fmt.Errorf("cannot link a library package"),
			Src: PackageErrorSrcLink,
		}
	}

	objects := p.objectFiles()
	if objects == nil {
		return &PackageError{
			Err: fmt.Errorf("objects are not found in the %q directory", p.Conf.ObjectsDir),
			Src: PackageErrorSrcLink,
		}
	}
	targetFilePath := filepath.Join(p.Dir, p.Conf.TargetFile)
	if _, err := os.Lstat(targetFilePath); os.IsExist(err) {
		os.Remove(targetFilePath)
	}
	args := []string{}
	for _, lib := range p.Conf.LibraryDirs {
		args = append(args, fmt.Sprintf("-L%s/", lib))
	}
	args = slices.Concat(args, p.Conf.LinkerFlags, p.baseCompileArgs(true), []string{
		"-Wl,--export-all",
		"-Wl,--no-entry",
	}, objects, []string{"-o", targetFilePath})
	cmd := exec.Command(
		"clang++",
		args...,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Print(string(output))
		return &PackageError{
			Err: fmt.Errorf("linking failed: %v", err),
			Src: PackageErrorSrcLink,
		}
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
		return &PackageError{
			Err: fmt.Errorf("cannot archive a program package"),
			Src: PackageErrorSrcArchive,
		}
	}
	objects := p.objectFiles()
	if objects == nil {
		return &PackageError{
			Err: fmt.Errorf("objects are not found in the %q directory", p.Conf.ObjectsDir),
			Src: PackageErrorSrcArchive,
		}
	}
	targetFilePath := filepath.Join(p.Dir, p.Conf.TargetFile)
	if _, err := os.Lstat(targetFilePath); os.IsExist(err) {
		os.Remove(targetFilePath)
	}
	args := slices.Concat([]string{
		"rcs",
		targetFilePath,
	}, objects)
	cmd := exec.Command(
		"llvm-ar",
		args...,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Print(string(output))
		return &PackageError{
			Err: fmt.Errorf("archiving failed: %v", err),
			Src: PackageErrorSrcLink,
		}
	}
	return nil
}

// GenerateCompilationDatabase is similar to [Package.Compile], but it inserts
// `bear --` before `clang++` to generate a compilation database that works
// with clangd.
//
// Returns an error if invoking `bear` failed (prints its combined output into
// the console on error).
//
// Returns [ErrInvalidPackageBuildMode] if [PackageBuildMode.Valid] returns
// false.
func (p *Package) GenerateCompilationDatabase() error {
	if !p.BuildMode.Valid() {
		return &PackageError{
			Err: ErrInvalidPackageBuildMode,
			Src: PackageErrorSrcCompile,
		}
	}
	if err := p.createObjsDir(); err != nil {
		return &PackageError{
			Src: PackageErrorSrcCompile,
			Err: fmt.Errorf("failed to create objects directory: %v", err),
		}
	}
	baseArgs := p.baseCompileArgs(false)
	for _, src := range p.Conf.Sources {
		finalSrc, objPath := p.convertSrcToObj(src, false)
		finalArgs := slices.Concat([]string{"--", "clang++"}, baseArgs, []string{
			"-c",
			"-o",
			objPath,
			finalSrc,
		})
		cmd := exec.Command(
			"bear",
			finalArgs...,
		)
		cmd.Dir = p.Dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Print(string(output))
			return &PackageError{
				Err: fmt.Errorf("generating compilation database failed: %v", err),
				Src: PackageErrorSrcCompilationDatabase,
			}
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
	return &PackageError{
		Err: ErrUnknownPackageType,
		Src: PackageErrorSrcPack,
	}
}

func (p *Package) convertSrcToObj(src string, addDir bool) (finalSrc string, objPath string) {
	finalSrc = src
	if addDir {
		finalSrc = filepath.Join(p.Dir, src)
	}
	objFilename, _ := strings.CutSuffix(src, ".cxx")
	objFilename += ".obj"
	objPath = filepath.Join(p.Dir, p.Conf.ObjectsDir, objFilename)
	return
}

func (p *Package) createObjsDir() error {
	objsDirPath := filepath.Join(p.Dir, p.Conf.ObjectsDir)
	if err := os.Mkdir(objsDirPath, os.ModeAppend|os.ModePerm); os.IsExist(err) {
		if err := os.RemoveAll(objsDirPath); err != nil {
			return err
		}
		if err := os.Mkdir(objsDirPath, os.ModeAppend|os.ModePerm); err != nil {
			return err
		}
	}
	return nil
}

func (p *Package) packProgram() error {
	if err := p.Link(); err != nil {
		return err
	}
	f, writer, err := p.createZipWriter()
	if err != nil {
		return err
	}
	defer f.Close()
	defer writer.Close()
	if err := p.writeBaseFilesToZip(writer); err != nil {
		return err
	}
	return writer.Close()
}

func (p *Package) packLibrary() error {
	if err := p.Archive(); err != nil {
		return err
	}
	f, writer, err := p.createZipWriter()
	if err != nil {
		return err
	}
	defer f.Close()
	defer writer.Close()
	if err := p.writeBaseFilesToZip(writer); err != nil {
		return err
	}
	for _, inc := range p.Conf.IncludeDirs {
		if err := p.packDirectory(writer, filepath.Join(p.Dir, inc)); err != nil {
			writer.Close()
			return err
		}
	}
	return writer.Close()
}

func (p *Package) createZipWriter() (*os.File, *zip.Writer, error) {
	f, err := os.Create(p.Conf.PackedFile)
	if err != nil {
		return nil, nil, &PackageError{
			Err: fmt.Errorf("creating %q failed: %v", p.Conf.PackedFile, err),
			Src: PackageErrorSrcPack,
		}
	}
	return f, zip.NewWriter(f), nil
}

func (p *Package) writeBaseFilesToZip(w *zip.Writer) error {
	if err := p.packFile(w, filepath.Join(p.Dir, p.Conf.TargetFile), p.Conf.TargetFile); err != nil {
		return &PackageError{
			Err: fmt.Errorf("packing %q file failed: %v", p.Conf.TargetFile, err),
			Src: PackageErrorSrcPack,
		}
	}
	if err := p.packFile(w, filepath.Join(p.Dir, "scintilla.toml"), "scintilla.toml"); err != nil {
		return &PackageError{
			Err: fmt.Errorf("packing scintilla.toml file failed: %v", err),
			Src: PackageErrorSrcPack,
		}
	}
	return nil
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
	objects, _ := filepath.Glob(filepath.Join(p.Dir, p.Conf.ObjectsDir, "*.obj"))
	return objects
}

func (p *Package) baseCompileArgs(addDir bool) []string {
	baseArgs := slices.Concat([]string{
		"--target=wasm32-unknown-unknown",
		"-nostdlib",
	}, p.Conf.CompilerFlags)

	for _, inc := range p.Conf.IncludeDirs {
		finalInc := inc
		if addDir {
			finalInc = filepath.Join(p.Dir, inc)
		}
		baseArgs = append(baseArgs, fmt.Sprintf("-I%s", finalInc))
	}

	switch p.BuildMode {
	case PackageBuildModeRelease:
		baseArgs = append(baseArgs, "-DRELEASE", "-O2")
	case PackageBuildModeDebug:
		baseArgs = append(baseArgs, "-DDEBUG")
	}

	return baseArgs
}

// UnpackDependencies iterates over the specified main package's dependencies archive files
// and unpacks them using [UnpackPackage]. Creates the specified temp directory
// if it does not exist. If a current unpacking dependency is a library,
// the function automatically appends its include directories to the main package,
// and links against its static library file.
func (p *PackageEnv) UnpackDependencies() error {
	if p.MainPackage.Conf == nil {
		return &PackageError{
			Err: ErrPackageConfNotParsed,
			Src: PackageErrorSrcUnpack,
		}
	}
	if err := p.createTmpDir(p.MainPackage.Dir); err != nil {
		return err
	}
	for _, file := range p.MainPackage.Conf.DependencyZipFiles {
		source := filepath.Join(p.MainPackage.Dir, file)
		dest := filepath.Join(p.MainPackage.Dir, p.tmpDependenciesPath(), filepath.Base(file))
		d, err := UnpackPackage(source, dest)
		if err != nil {
			return err
		}
		if err := d.ParseConfig(); err != nil {
			return err
		}
		if d.Conf.Type == PackageTypeLib {
			for _, inc := range d.Conf.IncludeDirs {
				str, _ := filepath.Rel(p.MainPackage.Dir, filepath.Join(dest, inc))
				p.MainPackage.Conf.IncludeDirs = append(p.MainPackage.Conf.IncludeDirs, str)
			}
			p.MainPackage.Conf.LibraryDirs = append(p.MainPackage.Conf.LibraryDirs, dest)
			// Check whether d.Conf.TargetFile is a valid library file, which means
			// it has the "lib" prefix and ".a" suffix. Later, we cut these off to make it a
			// linker flag in the main package configuration.
			if !strings.HasPrefix(d.Conf.TargetFile, "lib") || !strings.HasSuffix(d.Conf.TargetFile, ".a") {
				return &PackageError{
					Err: fmt.Errorf("invalid library target filename: must start with \"lib\" and end with \".a\""),
					Src: PackageErrorSrcUnpack,
				}
			}
			libraryName, _ := strings.CutPrefix(d.Conf.TargetFile, "lib")
			libraryName, _ = strings.CutSuffix(libraryName, ".a")
			p.MainPackage.Conf.LinkerFlags = append(p.MainPackage.Conf.LinkerFlags, fmt.Sprintf("-l%s", libraryName))
		}
		p.DependencyPackages = append(p.DependencyPackages, d)
	}
	return nil
}

// BuildDependencies builds and packs the specified main package's dependencies,
// changing the current working directory to p.MainPackage.Dir temporarily using [os.Chdir].
// BuildDependencies moves packed dependency files to p.MainPackage.Dir directory
// and appends them to p.MainPackage.Conf.DependencyZipFiles.
//
// This function must be used before [PackageEnv.UnpackDependencies].
func (p *PackageEnv) BuildDependencies() error {
	if p.MainPackage.Conf == nil {
		return &PackageError{
			Err: ErrPackageConfNotParsed,
			Src: PackageErrorSrcBuildDependencies,
		}
	}
	if err := p.createTmpDir(p.MainPackage.Dir); err != nil {
		return err
	}
	for _, path := range p.MainPackage.Conf.DependencyPaths {
		oldWd, _ := os.Getwd()
		if err := os.Chdir(p.MainPackage.Dir); err != nil {
			return &PackageError{
				Err: fmt.Errorf("setting working directory to %q failed: %v", p.MainPackage.Dir, err),
				Src: PackageErrorSrcBuildDependencies,
			}
		}

		d := NewPackage(path)
		if err := d.ParseConfig(); err != nil {
			return err
		}
		env := NewPackageEnv(d.Dir)
		env.MainPackage = d
		if err := env.BuildDependencies(); err != nil {
			return err
		}
		if err := env.UnpackDependencies(); err != nil {
			return err
		}
		d.BuildMode = p.MainPackage.BuildMode
		if err := d.Pack(); err != nil {
			return err
		}
		oldPath := d.Conf.PackedFile
		finalZipPath, _ := strings.CutPrefix(p.tmpZipPath(), p.MainPackage.Dir+string(os.PathSeparator))
		newPath := filepath.Join(finalZipPath, d.Conf.PackedFile)
		if err := os.Rename(oldPath, newPath); err != nil {
			return &PackageError{
				Err: fmt.Errorf("moving %q to %q failed: %v", oldPath, newPath, err),
				Src: PackageErrorSrcBuildDependencies,
			}
		}
		p.MainPackage.Conf.DependencyZipFiles = append(p.MainPackage.Conf.DependencyZipFiles, newPath)
		if err := os.Chdir(oldWd); err != nil {
			return &PackageError{
				Err: fmt.Errorf("setting working directory to %q back failed: %v", oldPath, err),
				Src: PackageErrorSrcBuildDependencies,
			}
		}
	}
	return nil
}

func (p *PackageEnv) createTmpDir(baseDir string) error {
	tmpPath := filepath.Join(baseDir, p.tmpPath())
	dependenciesPath := filepath.Join(tmpPath, "dependencies")
	zipPath := filepath.Join(tmpPath, "zip")
	mode := os.ModeAppend | os.ModePerm
	if _, err := os.Lstat(tmpPath); os.IsNotExist(err) {
		if err := os.Mkdir(tmpPath, mode); err != nil {
			return &PackageError{
				Err: fmt.Errorf("creating %q failed: %v", tmpPath, err),
				Src: PackageErrorSrcUnpack,
			}
		}
		if err := os.Mkdir(dependenciesPath, mode); err != nil {
			return &PackageError{
				Err: fmt.Errorf("creating %q failed: %v", dependenciesPath, err),
				Src: PackageErrorSrcUnpack,
			}
		}
		if err := os.Mkdir(zipPath, mode); err != nil {
			return &PackageError{
				Err: fmt.Errorf("creating %q failed: %v", zipPath, err),
				Src: PackageErrorSrcUnpack,
			}
		}
	}
	return nil
}

func (p *PackageEnv) tmpPath() string {
	return filepath.Join(".scintilla")
}

func (p *PackageEnv) tmpDependenciesPath() string {
	return filepath.Join(p.tmpPath(), "dependencies")
}

func (p *PackageEnv) tmpZipPath() string {
	return filepath.Join(p.tmpPath(), "zip")
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
		return nil, &PackageError{
			Err: fmt.Errorf("opening %q archive failed: %v", archivePath, err),
			Src: PackageErrorSrcUnpack,
		}
	}
	defer zipR.Close()

	if _, err := os.Lstat(destFolder); os.IsNotExist(err) {
		if err := os.Mkdir(destFolder, os.ModePerm|os.ModeAppend); err != nil {
			return nil, &PackageError{
				Err: fmt.Errorf("creating %q destination directory failed: %v", destFolder, err),
				Src: PackageErrorSrcUnpack,
			}
		}
	}

	var p *Package
	parsedPackageConf := false
	for _, file := range zipR.File {
		if file.Name == "scintilla.toml" {
			configFilepath := filepath.Join(destFolder, "scintilla.toml")
			r, err := file.Open()
			if err != nil {
				return nil, &PackageError{
					Err: fmt.Errorf("opening scintilla.toml in the archive failed: %v", err),
					Src: PackageErrorSrcUnpack,
				}
			}
			defer r.Close()
			f, err := os.Create(configFilepath)
			if err != nil {
				return nil, &PackageError{
					Err: fmt.Errorf("creating scintilla.toml in %q failed: %v", destFolder, err),
					Src: PackageErrorSrcUnpack,
				}
			}
			defer f.Close()
			if _, err := io.Copy(f, r); err != nil {
				return nil, &PackageError{
					Err: fmt.Errorf("copying scintilla.toml's data to physical scintilla.toml failed: %v", err),
					Src: PackageErrorSrcUnpack,
				}
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
		return nil, &PackageError{
			Err: fmt.Errorf("config scintilla.toml is not found in %q archive", archivePath),
			Src: PackageErrorSrcUnpack,
		}
	}

	if p.Conf.Type == PackageTypeLib {
		for _, file := range zipR.File {
			if file.FileInfo().IsDir() && slices.Contains(p.Conf.IncludeDirs, file.Name) {
				if err := unpackPackageDirectory(zipR, file.Name, filepath.Join(destFolder, file.Name)); err != nil {
					return nil, &PackageError{
						Err: fmt.Errorf("unpacking include directory %q to %q failed", file.Name, destFolder),
						Src: PackageErrorSrcUnpack,
					}
				}
			}
		}
	}

	for _, file := range zipR.File {
		if p.Conf.TargetFile == file.Name {
			targetPath := filepath.Join(destFolder, p.Conf.TargetFile)
			r, err := file.Open()
			if err != nil {
				return nil, &PackageError{
					Err: fmt.Errorf("opening target file %q failed: %v", file.Name, err),
					Src: PackageErrorSrcUnpack,
				}
			}

			f, err := os.Create(targetPath)
			if err != nil {
				return nil, &PackageError{
					Err: fmt.Errorf("creating physical target file %q failed: %v", targetPath, err),
					Src: PackageErrorSrcUnpack,
				}
			}

			if _, err := io.Copy(f, r); err != nil {
				return nil, &PackageError{
					Err: fmt.Errorf("copying %q's data to physical %q failed: %v", err, file.Name, targetPath),
					Src: PackageErrorSrcUnpack,
				}
			}
			break
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
