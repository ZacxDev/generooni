# Generooni

## Problem
Code generation tools are essential in modern development but often lead to slow build times, complex dependencies, and inconsistent results. Developers struggle with lengthy codegen processes, intricate execution orders, and the need for multiple runs to ensure all generated code is up-to-date.

## Solution
Generooni is a powerful codegen orchestration system that isolates, maps dependencies, and caches code generation processes. It creates a hermetic codegen pipeline, dramatically reducing generation times while ensuring code consistency and correctness.

---

## Overview

Generooni is designed to solve the challenges introduced by heavy use of code generation tools. It offers a flexible and efficient approach to managing codegen processes, addressing issues such as:

- Slow codegen times as more jobs are added
- Interdependencies requiring complex execution order or multiple runs
- Inconsistent or outdated generated code

By isolating and mapping codegen jobs, Generooni enables a hermetic codegen pipeline that can be cached both locally and in CI systems. This brings codegen times to near-zero while still guaranteeing the code is in the correct state.

## Features

- **Dependency Mapping**: Automatically detects and manages dependencies between codegen tasks.
- **Caching**: Implements intelligent caching to avoid unnecessary regeneration of unchanged code.
- **Isolation**: Runs each codegen job in isolation to prevent conflicts and ensure consistency.
- **Flexible Configuration**: Uses a Starlark-based configuration system for powerful and customizable setups.
- **CI Integration**: Easily integrates with CI/CD pipelines for efficient builds and deployments.

## Getting Started

### Installation

```bash
go get github.com/your-org/generooni
```

### Basic Usage

1. Create a `generooni.star` configuration file in your project root.
2. Define your codegen jobs and their dependencies.
3. Run Generooni:

```bash
generooni generate
```

### Example Configuration

```python
# generooni.star

def filesystem_target(cmd, outputs, dependencies=None, dependency_sync_patterns=None):
    return {
        "cmd": cmd,
        "outputs": outputs,
        "dependencies": dependencies or [],
        "dependency_sync_patterns": dependency_sync_patterns or [],
    }

config = {
    "protobuf": filesystem_target(
        cmd = "protoc --go_out=. --go-grpc_out=. *.proto",
        outputs = ["*.pb.go"],
        dependencies = ["*.proto"],
    ),
    "graphql": filesystem_target(
        cmd = "gqlgen generate",
        outputs = ["graph/generated.go", "graph/model/models_gen.go"],
        dependencies = ["graph/*.graphqls", "gqlgen.yml"],
        target_deps = ["protobuf"],
    ),
}
```

## Advanced Features

- **Partial Jobs**: Support for jobs that modify existing files.
- **Custom Caching**: Fine-grained control over what gets cached and how.
- **Parallel Execution**: Automatically runs independent jobs in parallel for faster execution.

