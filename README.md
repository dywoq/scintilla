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

Scintilla is suitable for:

- **creating a standard library/runtime from scratch in guest C++ code.**
- **adapting an existing implementation of the official C++ standard library to Scintilla.**

Since Scintilla automatically disables standard library and enables freestanding mode, these goals
are possible. Use it when you want to build your own ecosystem without relying on ready solutions.

# Installation

Scintilla provides a CLI tool to manage Scintilla packages. You can install it using the
following command:

```
go install github.com/dywoq/scintilla/cmd/scintilla@latest
```

# Documentation

The folder [`docs/`](https://github.com/dywoq/scintilla/tree/main/docs) gives a comprehensive description
of available Scintilla commands and configuration keys. Its public programming interface is available
in https://pkg.go.dev.

# License

```
Copyright 2026 dywoq - Apache License 2.0
```
