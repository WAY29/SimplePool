package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/swaggo/swag/gen"
)

const (
	outputDir    = "internal/httpapi/openapi"
	outputFile   = "openapi.json"
	mainAPIFile  = "internal/httpapi/router.go"
	searchDir    = "."
	swaggerFile  = "swagger.json"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := gen.New().Build(&gen.Config{
		SearchDir:          searchDir,
		MainAPIFile:        mainAPIFile,
		OutputDir:          outputDir,
		OutputTypes:        []string{"json"},
		ParseDepth:         100,
		ParseInternal:      true,
		ParseDependency:    1,
		ParseFuncBody:      true,
		GeneratedTime:      false,
		PackageName:        "openapi",
		PropNamingStrategy: "snakecase",
		ParseGoList:        true,
	}); err != nil {
		return fmt.Errorf("generate swagger json: %w", err)
	}

	swaggerPath := filepath.Join(outputDir, swaggerFile)
	raw, err := os.ReadFile(swaggerPath)
	if err != nil {
		return fmt.Errorf("read swagger json: %w", err)
	}

	var doc2 openapi2.T
	if err := json.Unmarshal(raw, &doc2); err != nil {
		return fmt.Errorf("decode swagger json: %w", err)
	}

	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return fmt.Errorf("convert swagger to openapi3: %w", err)
	}

	doc3.OpenAPI = "3.0.3"
	normalizeSpec(doc3)

	data, err := json.MarshalIndent(doc3, "", "  ")
	if err != nil {
		return fmt.Errorf("encode openapi json: %w", err)
	}

	if err := os.WriteFile(filepath.Join(outputDir, outputFile), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write openapi json: %w", err)
	}

	if err := os.Remove(swaggerPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove swagger json: %w", err)
	}

	return nil
}

func normalizeSpec(doc *openapi3.T) {
	if doc.Info != nil {
		if doc.Info.Title == "" {
			doc.Info.Title = "SimplePool API"
		}
		if doc.Info.Version == "" {
			doc.Info.Version = "0.1.0"
		}
	}
	if doc.Paths == nil {
		doc.Paths = openapi3.NewPaths()
	}
}
