package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ZacxDev/generooni/config"
	"github.com/ZacxDev/generooni/executor"
	"github.com/ZacxDev/generooni/fs"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "generooni",
		Usage: "A code generation and dependency management tool",
		Commands: []*cli.Command{
			{
				Name:   "generate",
				Usage:  "Generate code based on the configuration",
				Action: generateAction,
			},
			{
				Name:   "map-dependencies",
				Usage:  "Map dependencies and generate generooni-deps.star file",
				Action: mapDependenciesAction,
			},
		},
		Action: func(c *cli.Context) error {
			color.Yellow("Warning: Running generooni without a subcommand is deprecated. Please use 'generooni generate' instead.")
			return generateAction(c)
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generateAction(c *cli.Context) error {
	return run()
}

func mapDependenciesAction(c *cli.Context) error {
	fileSystem := &fs.RealFileSystem{}
	cmdExecutor := &executor.RealCommandExecutor{}
	cacheManager := executor.NewCacheManager(fileSystem, ".generooni-cache")
	lockManager := executor.NewLockFileManager(fileSystem)

	targetExecutor := executor.NewTargetExecutor(fileSystem, cmdExecutor, cacheManager, lockManager)

	targets, err := config.ParseStarlarkConfig("generooni.star")
	if err != nil {
		return errors.Wrap(err, "failed to parse Starlark config")
	}

	for _, target := range targets {
		targetExecutor.AddTarget(target)
	}

	return targetExecutor.MapDependencies()
}

func run() error {
	fileSystem := &fs.RealFileSystem{}
	cmdExecutor := &executor.RealCommandExecutor{}
	cacheManager := executor.NewCacheManager(fileSystem, ".generooni-cache")
	lockManager := executor.NewLockFileManager(fileSystem)

	targetExecutor := executor.NewTargetExecutor(fileSystem, cmdExecutor, cacheManager, lockManager)

	if err := targetExecutor.Initialize(); err != nil {
		return errors.Wrap(err, "failed to initialize target executor")
	}

	targets, err := config.ParseStarlarkConfig("generooni.star")
	if err != nil {
		return errors.Wrap(err, "failed to parse Starlark config")
	}

	for _, target := range targets {
		targetExecutor.AddTarget(target)
	}

	if err := targetExecutor.ExecuteTargets(); err != nil {
		return errors.Wrap(err, "failed to execute targets")
	}

	fmt.Println("All targets completed successfully.")
	return nil
}

func printErrorWithFix(err error) {
	color.Red("Error: %v", err)

	switch errors.Cause(err) {
	case os.ErrNotExist:
		color.Yellow("Fix: Ensure that the required file or directory exists and you have the necessary permissions.")
	case os.ErrPermission:
		color.Yellow("Fix: Check your file permissions and ensure you have the necessary access rights.")
	default:
		color.Yellow("Fix: Please check the error message above and ensure all configurations are correct.")
	}

	fmt.Println("If the problem persists, please report this issue to the development team.")
}
