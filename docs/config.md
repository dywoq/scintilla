# Configuration

The configuration file (`scintilla.toml`) consists of the package's information. It relies on the
**TOML (v1.1.0)** configuration format.

# Keys

- `target_file` (`string`): What to archive/link after compilation of object files.
- `packed_file` (`string`): Where to write the package's contents. This file is a ZIP file.
  It is created if it does not exist.
- `type` (`string`): The package's type. **Allowed values**: `lib`, `program`.
- `sources` (`[]string`): The package's sources. At the moment, Scintilla supports
  compilation of source files, placed at the package's root folder.
- `objects_dir` (`string`): The directory where compiled object files are placed.
- `compiler_flags` (`[]string`): Additional custom compiler flags.
- `linker_flags` (`[]string`): Additional custom linker flags.
- `include_dirs` (`[]string`): Include directories of a package.
- `library_dirs` (`[]string`): Library directories of a package.
- `dependency_zip_files` (`[]string`): ZIP files, packed using the `scintilla pack`
  command (documented in [`docs/cli.md`](./cli.md)).
- `dependency_paths` (`[]string`): Specifies the dependencies which are built first before
  the main package.
