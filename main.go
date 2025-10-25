package main

import (
	"fmt"
	"gqlc/compiler"
	"gqlc/config"
	"gqlc/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

var buildVersion = "(devel)"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			if err := initConfig(); err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
				return
			}
			return
		case "version", "--version", "-v":
			if buildVersion == "dev" {
				if bi, ok := debug.ReadBuildInfo(); ok {
					buildVersion = bi.Main.Version
				}
			}
			fmt.Printf("gqlc %s\n", buildVersion)
			return
		}
	} else {
		startedAt := time.Now()
		if err := run(); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
			return
		}
		fmt.Printf("Finished in %s\n", time.Since(startedAt))
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Load operations using the fs utility
	operationsSrc, err := fs.CollectGraphQLFiles(cfg.Input.Operations)
	if err != nil {
		return fmt.Errorf("failed to load operations: %w", err)
	}
	defer func() {
		for _, f := range operationsSrc {
			if f != nil {
				_ = f.Close()
			}
		}
	}()

	outDir := cfg.Output.Location
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outSchemaName := fmt.Sprintf("schema%s.%s", cfg.Output.Suffix, cfg.Output.FileExtension())
	outSchemaPath := filepath.Join(outDir, outSchemaName)
	outSchemaFile, err := os.Create(outSchemaPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outSchemaFile.Close()

	outOp := filepath.Join(outDir, fmt.Sprintf("operations%s.%s", cfg.Output.Suffix, cfg.Output.FileExtension()))
	outOpFile, err := os.Create(outOp)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outOpFile.Close()

	err = compiler.Compile(cfg, operationsSrc, outSchemaFile, outSchemaName, outOpFile)
	if err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}

	return nil
}

func initConfig() error {
	ext := "yaml"
	if len(os.Args) > 2 {
		for _, arg := range os.Args[2:] {
			switch arg {
			case "json", "xml", "toml", "yaml", "yml":
				ext = arg
			case "help":
				fmt.Println("Usage: gqlc init [yaml|yml|toml|json|xml]")
				return nil
			default:
				return fmt.Errorf("unsupported extension %s (supported: yaml|yml|toml|json|xml)", arg)
			}
		}
	}
	config := config.New()
	if err := config.SaveAs(ext); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}
