# Scintilla

Scintilla represents a custom scripting infrastructure, which is based on WebAssembly and C++.
It is responsible for loading several scripts at once and running them in one function call.
Scintilla provides a way to register and expose your host functions to guest code. Along with that,
Scintilla provides functionality to pack, compile and archive/link your Scintilla packages.

# Purpose

You might use Scintilla to **develop games and use C++ as a scripting language**. Since its
programming interface is available in Golang only, you can use it in your Golang-based games
powered by [Ebiten](https://github.com/hajimehoshi/ebiten). However, Scintilla is flexible and there may
be other uses.

# Installation

Scintilla provides a CLI tool to manage Scintilla packages. You can install it using the
following command:

```
go install github.com/dywoq/scintilla/cmd/scintilla@latest
```

# Documentation

The folder `docs/` gives a comprehensive description of available Scintilla commands and
features.

# License

```
Copyright 2026 dywoq - Apache License 2.0
```
