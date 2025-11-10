package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/projectdiscovery/alterx"
	"github.com/projectdiscovery/alterx/internal/runner"
	"github.com/projectdiscovery/gologger"
)

func main() {
	cliOpts := runner.ParseFlags()

	alterOpts := alterx.Options{
		Domains:       cliOpts.Domains,
		Patterns:      cliOpts.Patterns,
		Payloads:      cliOpts.Payloads,
		Limit:         cliOpts.Limit,
		Enrich:        cliOpts.Enrich,
		MaxSize:       cliOpts.MaxSize,
		DedupeResults: true, // Enable deduplication by default
	}

	if cliOpts.PermutationConfig != "" {
		// read config
		config, err := alterx.NewConfig(cliOpts.PermutationConfig)
		if err != nil {
			gologger.Fatal().Msgf("failed to read config file '%s': %v", cliOpts.PermutationConfig, err)
		}
		if len(config.Patterns) > 0 {
			alterOpts.Patterns = config.Patterns
		}
		if len(config.Payloads) > 0 {
			alterOpts.Payloads = config.Payloads
		}
	}

	// Configure output writer
	var output io.Writer
	var outputFile *os.File
	if cliOpts.Output != "" {
		fs, err := os.OpenFile(cliOpts.Output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			gologger.Fatal().Msgf("failed to open output file '%s': %v", cliOpts.Output, err)
		}
		output = fs
		outputFile = fs
		defer func() {
			if err := fs.Close(); err != nil {
				gologger.Error().Msgf("failed to close output file: %v", err)
			}
		}()
	} else {
		output = os.Stdout
	}

	// Create new alterx instance with options
	m, err := alterx.New(&alterOpts)
	if err != nil {
		gologger.Fatal().Msgf("failed to initialize alterx: %v", err)
	}

	if cliOpts.Estimate {
		gologger.Info().Msgf("Estimated Payloads (including duplicates): %d", m.EstimateCount())
		return
	}

	// Setup context with cancellation support for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		gologger.Warning().Msg("Received interrupt signal, stopping...")
		cancel()
	}()

	if err = m.ExecuteWithWriter(ctx, output); err != nil {
		if err == context.Canceled {
			gologger.Warning().Msg("Operation cancelled by user")
			if outputFile != nil {
				outputFile.Close()
			}
			os.Exit(130) // Standard exit code for SIGINT
		}
		gologger.Fatal().Msgf("failed to generate permutations: %v", err)
	}
}
