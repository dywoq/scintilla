// Copyright 2026 Bellus Development Team - All Rights Reserved

package scintilla

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// PackageType identifies the type of a Scintilla package.
type PackageType string

// PackageConf contains the TOML configuration.
type PackageConf struct {
	Type       PackageType `toml:"type"`
	Sources    []string    `toml:"sources"`
	ObjectsDir string      `toml:"objects_dir"`
	Flags      []string    `toml:"flags"`
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
