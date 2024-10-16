[![Go Report Card](https://goreportcard.com/badge/github.com/ZacxDev/generooni)](https://goreportcard.com/report/github.com/ZacxDev/generooni)
# Generooni

**Effortless Code Generation Orchestration**

## Introduction

Generooni is a powerful tool that streamlines your code generation processes. By intelligently managing dependencies, caching outputs, and isolating tasks, it ensures your codegen pipeline is fast, consistent, and correct.

## Why Use Generooni?

Modern development often relies on multiple code generation tools, leading to:

- **Slow Build Times**: As more codegen tasks are added, builds become sluggish.
- **Complex Dependencies**: Managing execution order of interdependent tasks can become a nightmare.
- **Inconsistent Code**: Generated code may be outdated or conflicting, especially if checked in.
- **Multiple Runs Needed**: Ensuring all code is up-to-date often requires several passes, increasing build times.

**Generooni** solves these problems by:

- **Mapping Dependencies**: Automatically detects and manages task interdependencies.
- **Caching Outputs**: Avoids regenerating code when inputs haven't changed.
- **Isolating Tasks**: Runs each job separately to prevent conflicts.
- **Parallel Execution**: Executes independent tasks concurrently for speed.
- **State Verification**: Uses a lockfile to ensure generated code files are in the correct state; restoring or regenerating them if needed.

## Key Features

- **Simple Configuration**: Define your codegen tasks using an easy-to-write Starlark file.
- **Intelligent Caching**: Speeds up builds by reusing unchanged outputs.
- **Automatic Dependency Management**: Ensures tasks run in the correct order.
- **CI/CD Friendly**: Seamlessly integrates into your existing pipelines by checking in .generooni-cache and generooni.lock files, or by externally caching/pulling them in

## Getting Started

> ⚠️ **Warning**: Generooni is not ready for production use. It's a work in progress and may contain bugs or incomplete features.  If you are keen to try it out, it is recommended that you use generooni locally, while still running codegen tasks directly in CI.

### Installation

Install Generooni using Go:

```bash
go get github.com/ZacxDev/generooni
```

### Basic Usage

1. **Create a Configuration File**

   Add a `generooni.star` file to your project's root directory.

2. **Define Codegen Tasks**

   Specify your code generation commands, outputs, and dependencies.

3. **Run Generooni**

   Generate your code with:

   ```bash
   generooni map-dependencies # sync job dependency files based on dependency_sync_patterns
   generooni generate
   ```

### Example Configuration

```python
# generooni.star

load("generooni-deps.star", "filesystem_target_dependency_map")

def get_dependencies(target_name):
    return filesystem_target_dependency_map.get(target_name, [])

def filesystem_target(cmd, outputs, dependencies=None, target_deps=None):
    return {
        "cmd": cmd,
        "outputs": outputs,
        "dependencies": dependencies or [],
        "target_deps": target_deps or [],
    }

config = {
    "protobuf": filesystem_target(
        cmd="protoc --go_out=. --go-grpc_out=. *.proto", # The command to run
        outputs=["*.pb.go"], # What files do we expect to be generated. used for caching, make sure this doesn't match any files you might change, because they will be overwritten
        dependencies = get_dependencies("protobuf"), # pull in generated dependency map
        dependency_sync_patterns=["*.proto"], # used with map-dependencies subcommand to generate job-inputs dependency map as used above
    ),
    "graphql": filesystem_target(
        cmd="gqlgen generate",
        outputs=["graph/generated.go", "graph/model/models_gen.go"],
        dependencies = get_dependencies("graphql"),
        dependency_sync_patterns=["graph/*.graphqls", "gqlgen.yml"],
        target_deps=["protobuf"], # other targets this target depends on
    ),
}
```

In this example:

- We import the dependency mappings generated by `generooni map-dependencies`, and setup a helper function to extract them
- **`protobuf` Task**: Generates Go code from `.proto` files.
- **`graphql` Task**: Generates GraphQL code and depends on the `protobuf` task.

### Running the Generator

Execute:

```bash
generooni map-dependencies # sync job dependency files based on dependency_sync_patterns
generooni generate
```

Generooni will:

- Analyze tasks and dependencies.
- Execute tasks in the correct order.
- Cache outputs to speed up future runs.

## Advanced Usage

### Partial Tasks

Support tasks that modify existing files by specifying affected files.

### Custom Caching

Control caching behavior for specific tasks and outputs.

### Parallel Execution

Automatically runs independent tasks in parallel to reduce build times.

## CI/CD Integration

Include Generooni in your continuous integration pipeline to ensure consistent code generation across all environments. Benefit from cached outputs to accelerate build times.

## Contributing

We welcome contributions!

