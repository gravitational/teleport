package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/trace"
)

func main() {
	err := run()
	if err != nil {
		slog.ErrorContext(context.Background(), "error running command", "error", err)
		os.Exit(1)
	}
}

func buildSubcommandRunners() ([]Runner, error) {
	ar, err := NewAptRunner()
	if err != nil {
		return nil, trace.Wrap(err, "failed to instantiate new APT runner")
	}

	yr, err := NewYumRunner()
	if err != nil {
		return nil, trace.Wrap(err, "failed to instantiate new YUM runner")
	}

	// These should be sorted alphabetically by `Name()`
	return []Runner{
		*ar,
		*yr,
	}, nil
}

func run() error {
	subcommands, err := buildSubcommandRunners()
	if err != nil {
		return trace.Wrap(err, "failed to build subcommand runners")
	}

	// 2 = program name + subcommand
	if len(os.Args) < 2 {
		logHelp(subcommands)
		return trace.Errorf("subcommand not provided")
	}

	subcommandName := strings.ToLower(os.Args[1])
	for _, subcommand := range subcommands {
		if strings.ToLower(subcommandName) != subcommand.Name() {
			continue
		}

		// 2 = program name + subcommand, skip them and get subcommand arguments
		args := os.Args[2:]
		err := subcommand.Init(args)
		if err != nil {
			return trace.Wrap(err, "failed to initialize runner for subcommand %q", subcommandName)
		}

		setupLogger(subcommand.GetLoggerConfig())
		err = subcommand.Run()
		if err != nil {
			return trace.Wrap(err, "failed to run subcommand %q", subcommandName)
		}

		return nil
	}

	if subcommandName == "-h" {
		logHelp(subcommands)
		return nil
	}

	logHelp(subcommands)
	return trace.Errorf("no subcommands found matching %q", subcommandName)
}

func logHelp(subcommands []Runner) {
	executableName := os.Args[0]
	fmt.Printf("%s - OS package repo builder/updater\n", executableName)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println()
	for _, subcommand := range subcommands {
		fmt.Printf("\t%s\t%s\n", subcommand.Name(), subcommand.Info())
	}
	fmt.Println()
	fmt.Printf("Use \"%s <command> -h\" for more information about a command.\n", executableName)
	fmt.Println()
}

func setupLogger(config *LoggerConfig) {
	var level slog.LevelVar
	level.Set(slog.LevelInfo)

	switch config.logLevel {
	case PanicLevel, FatalLevel:
		// The value here isn't particularly important, it just needs
		// to be higher than error level to preserve compatibility.
		level.Set(slog.LevelError + 1)
	case ErrorLevel:
		level.Set(slog.LevelError)
	case WarnLevel:
		level.Set(slog.LevelWarn)
	case InfoLevel:
		level.Set(slog.LevelInfo)
	case DebugLevel:
		level.Set(slog.LevelDebug)
	case TraceLevel:
		// The value here isn't particularly important, it just needs
		// to be lower than debug level to preserve compatibility.
		level.Set(slog.LevelDebug - 1)
	}

	if config.logJSON {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: &level})))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level})))
	}

	slog.DebugContext(context.Background(), "Setup logger with config", "config", config)
}
