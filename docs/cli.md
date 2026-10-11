# CLI

This document describes available commands and their flags in Scintilla. As stated in the
[README.md](https://github.com/dywoq/scintilla/blob/main/README.md).

# Commands

## `build`

**Usage**:

```
scintilla build [package_dir]
```

**Description**: This command opens the package. Before building the main package,
`build` builds and unpacks dependencies, automatically specifying include directories and
libraries to the main package's compiler/linker flags. Depending on the package's type:

- `lib`: Archives the main package.
- `program`: Links the main package.

Consider that your local machine must have [LLVM Project](https://github.com/llvm/llvm-project) installed
(the minimal version is `22.1`).

**Flags**:

- `build_mode`: This flag pre-defines a set of corresponding macros and flags, depending on the
  value of this flag. **Allowed values**:
  - `release`:
    - **Defined macros**: `RELEASE`
    - **Compiler flags**: `-O2`
  - `debug`:
    - **Defined macros**: `DEBUG`

## `pack`

**Usage**:

```
scintilla pack [package_dir]
```

**Description**: This command runs the same operations as `build`. After retrieving the target file,
`pack` creates the specified archive ZIP file (`packed_file`) and writes the target file and configuration
file (`scintilla.toml`) to `packed_file`. If the package is a library, `pack` writes the specified include
directories to the ZIP file.

**Flags**:

- `build_mode`: See the flag `build_mode`'s description in the `build` heading.

## `compiledb`

**Usage**:

```
scintilla compiledb [package_dir]
```

**Description**: This command runs the same operations as `build`. However, it generates a compilation
database, relying on [Bear](https://github.com/rizsotto/bear) and temporarily setting the current working
directory to the specified package directory (`package_dir`).

**Flags**:

- `build_mode`: See the flag `build_mode`'s description in the `build` heading.

## `init`

**Usage**:

```
scintilla init [package_dir]
```

**Description**: This command creates the specified package directory and initializes it. There are two
versions of package's layout, which depend on the flag `type`'s value. To see the difference, it is
recommended to go to the [`_example`](https://github.com/dywoq/scintilla/tree/main/_example) folder
and enter the configuration files of the library (`printLib`) and program (`program`).

**Flags**:

- `type`: Decides the type of a package. `init` modifies the package's
  configuration and layout, reading the flag's value. **Allowed values**: `lib` and `program`.
