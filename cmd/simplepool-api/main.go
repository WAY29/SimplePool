package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/WAY29/SimplePool/internal/app"
	"github.com/WAY29/SimplePool/internal/config"
	"github.com/gin-gonic/gin"
)

type options struct {
	ConfigPath string
	Debug      bool
}

const cliName = "SimpleTool"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run(parent context.Context, args []string) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	opts, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = fmt.Fprint(os.Stdout, usageText())
			return nil
		}

		return fmt.Errorf("parse args failed: %w", err)
	}

	setGinMode(opts.Debug)

	cfg, err := loadConfig(opts)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}

	instance, err := app.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("bootstrap app failed: %w", err)
	}

	if err := instance.Start(); err != nil {
		return fmt.Errorf("start app failed: %w", err)
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := instance.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown app failed: %w", err)
	}

	return nil
}

func loadConfig(opts options) (config.Config, error) {
	if err := loadEnvFile(opts.ConfigPath); err != nil {
		return config.Config{}, err
	}

	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, err
	}
	cfg.Debug = cfg.Debug || opts.Debug
	return cfg, nil
}

func parseArgs(args []string) (options, error) {
	var opts options

	flags := flag.NewFlagSet(cliName, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.ConfigPath, "config", ".env", "path to env config file")
	flags.BoolVar(&opts.Debug, "debug", false, "enable gin debug mode")

	if err := flags.Parse(args); err != nil {
		return options{}, err
	}

	if flags.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	return opts, nil
}

func usageText() string {
	var builder strings.Builder
	flags := flag.NewFlagSet(cliName, flag.ContinueOnError)
	flags.SetOutput(&builder)
	flags.String("config", ".env", "path to env config file")
	flags.Bool("debug", false, "enable gin debug mode")
	flags.PrintDefaults()
	return "Usage of " + cliName + ":\n" + builder.String()
}

func setGinMode(debug bool) {
	if debug {
		gin.SetMode(gin.DebugMode)
		return
	}

	gin.SetMode(gin.ReleaseMode)
}

func loadEnvFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config file %q: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("parse config file %q line %d: missing '='", path, lineNumber)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("parse config file %q line %d: empty key", path, lineNumber)
		}

		value, err := parseEnvValue(strings.TrimSpace(rawValue))
		if err != nil {
			return fmt.Errorf("parse config file %q line %d: %w", path, lineNumber, err)
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %q from config file %q: %w", key, path, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	return nil
}

func parseEnvValue(raw string) (string, error) {
	if len(raw) < 2 {
		return raw, nil
	}

	if raw[0] == '"' && raw[len(raw)-1] == '"' {
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted value: %w", err)
		}

		return value, nil
	}

	if raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return raw[1 : len(raw)-1], nil
	}

	return raw, nil
}
