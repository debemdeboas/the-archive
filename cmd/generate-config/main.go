package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"github.com/debemdeboas/the-archive/internal/config"
)

func main() {
	// Create a config with defaults applied
	cfg := &config.Config{}
	config.ApplyDefaults(cfg)

	// Marshal to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating YAML: %v\n", err)
		os.Exit(1)
	}

	// Add header comment
	header := "# The Archive Configuration Example\n# Copy this file to config.yaml and customize as needed\n\n"
	output := header + string(yamlData)

	// Write to file or stdout
	outputFile := "config.example.yaml"
	if len(os.Args) > 1 {
		outputFile = os.Args[1]
	}

	if outputFile == "-" {
		fmt.Print(output)
	} else {
		err = os.WriteFile(outputFile, []byte(output), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Generated example config: %s\n", outputFile)
	}
}