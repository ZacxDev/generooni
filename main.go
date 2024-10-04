package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/ZacxDev/generooni/config"
	"github.com/ZacxDev/generooni/executor"
	"github.com/bmatcuk/doublestar/v4"
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
	targets, err := config.ParseStarlarkConfig("generooni.star")
	if err != nil {
		return errors.Wrap(err, "failed to parse Starlark config")
	}

	dependencyMap := make(map[string][]string)

	for name, target := range targets {
		dependencyMap[name] = target.Dependencies

		for _, pattern := range target.DependencySyncPatterns {
			matches, err := doublestar.FilepathGlob(pattern)
			if err != nil {
				return errors.Wrapf(err, "error expanding glob pattern %s", pattern)
			}

		matchLoop:
			for _, match := range matches {
				fileInfo, err := os.Stat(match)
				if err != nil {
					return errors.Wrapf(err, "error getting file info for %s", match)
				}
				if !fileInfo.IsDir() {
					for _, dep := range dependencyMap[name] {
						if dep == match {
							continue matchLoop
						}
					}

					dependencyMap[name] = append(dependencyMap[name], match)
				}
			}
		}
	}

	// Expand glob patterns in DependencySyncPatterns and update Dependencies
	starlarkCode := generateStarlarkCode(dependencyMap)

	if err := os.WriteFile("generooni-deps.star", []byte(starlarkCode), 0644); err != nil {
		return errors.Wrap(err, "failed to write generooni-deps.star file")
	}

	fmt.Println("Successfully generated generooni-deps.star file")
	return nil
}

func generateStarlarkCode(dependencyMap map[string][]string) string {
	jsonBytes, _ := json.MarshalIndent(dependencyMap, "", "  ")
	return fmt.Sprintf(`# This file is auto-generated. Do not edit manually.

filesystem_target_dependency_map = %s
`, string(jsonBytes))
}

func run() error {
	executor := executor.NewTargetExecutor()

	if err := os.MkdirAll(executor.CacheDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create cache directory")
	}

	if err := executor.LoadLockFile(); err != nil {
		return errors.Wrap(err, "failed to load lock file")
	}

	targets, err := config.ParseStarlarkConfig("generooni.star")
	if err != nil {
		return errors.Wrap(err, "failed to parse Starlark config")
	}

	for _, target := range targets {
		executor.AddTarget(target)
	}

	if err := executor.ExecuteTargets(); err != nil {
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
