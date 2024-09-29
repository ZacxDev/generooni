# Generooni

Generooni is a powerful and flexible codegen system designed to isolate, optimize, and cache code generation processes. Originally conceived as a faster alternative to `go generate`, Generooni has evolved into a language-agnostic tool that can enhance any development workflow involving code generation or complex build steps.

## Core Design Goals

1. **Isolation of Code Generation**: Generooni treats each code generation step as an isolated process, managing its inputs, outputs, and dependencies separately. This isolation allows for fine-grained control and optimization of each step in your codegen pipeline.

2. **Powerful Caching**: By mapping inputs to outputs for each codegen step, Generooni implements an intelligent caching system. This means that code is only regenerated when necessary, significantly speeding up build times in large projects.

3. **Incremental Regeneration**: Thanks to its caching system and dependency tracking, Generooni can perform incremental codegen, regenerating only the parts of your codebase that have been affected by changes.

4. **Language Agnostic**: While originally designed for Go, Generooni works with any programming language or build system. It achieves this by focusing on running commands and managing their inputs and outputs, rather than being tied to any specific language constructs.

5. **Flexible Configuration**: Using a Starlark configuration file, Generooni allows you to define complex codegen processes with ease, accommodating a wide variety of project structures requirements.

## Features

- **Starlark Configuration**: Define your codegen targets using a powerful and flexible Starlark configuration file.
- **Dependency Management**: Automatically handles dependencies between targets, ensuring correct codegen order.
- **Intelligent Caching**: Maps inputs to outputs for each target, enabling fast, incremental generation.
- **Parallel Execution**: Executes independent targets in parallel for faster generation
- **Interactive UI**: Provides a real-time, interactive terminal UI to monitor generation progress.
- **Detailed Logging**: Offers comprehensive logging capabilities for debugging and auditing.

## Installation

To install Generooni, ensure you have Go installed on your system, then run:

```
go get github.com/ZacxDev/generooni
```

## Usage

1. Create a `generooni.star` file in your project root to define your codegen targets.
2. Run Generooni:

```
generooni
```

Use the `--log-out` flag to log all job command output to stdout:

```
generooni --log-out
```

## Configuration

Generooni uses a Starlark configuration file named `generooni.star`. Here's an example of how to define targets:

```python
def filesystem_target(cmd, outputs, dependencies=None, target_deps=None):
    return {
        "cmd": cmd,
        "outputs": outputs,
        "dependencies": dependencies or [],
        "target_deps": target_deps or [],
    }

config = {
    "protobuf": filesystem_target(
        cmd = "protoc --go_out=. --go_opt=paths=source_relative *.proto",
        outputs = ["*.pb.go"],
        dependencies = ["*.proto"],
    ),
    "swagger": filesystem_target(
        cmd = "swag init",
        outputs = ["docs/swagger.json", "docs/swagger.yaml"],
        dependencies = ["**/*.go"],
        target_deps = ["protobuf"],
    ),
    # Add more targets here...
}
```

In this example, the `protobuf` target generates Go code from Protocol Buffer definitions, and the `swagger` target generates Swagger documentation, depending on the `protobuf` target.

## How It Works

1. **Dependency Resolution**: Generooni analyzes the dependency graph defined in your configuration file.
2. **Input Hashing**: For each target, Generooni creates a hash of all input files and the command to be executed.
3. **Cache Checking**: This hash is checked against a cache of previous generations.
4. **Execution**: If the hash doesn't match a cached result, the command is executed.
5. **Output Caching**: The results are cached, mapping the input hash to the generated outputs.
6. **Incremental Builds**: On subsequent runs, only targets with changed inputs or dependencies are re-executed.

This process ensures that code generation steps are only performed when necessary, significantly speeding up codegen times, especially in large projects with complex generation steps.

## Interactive UI

Generooni provides an interactive terminal UI that displays real-time status of your codegen targets. Use the following keys to navigate:

- `q` or `Ctrl+C`: Quit the program
- `Enter` or `Space`: Toggle log view for the selected target
- `Up/Down` or `j/k`: Navigate between targets
- `Esc`: Return to the main view from the log view

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

