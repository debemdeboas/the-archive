package main

import (
	"fmt"
	"os"
	"time"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

func main() {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Logger()

	// Create a config with defaults applied
	cfg := &config.Config{}
	config.ApplyDefaults(cfg)

	// Marshal to YAML
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Error generating YAML")
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
			log.Fatal().Err(err).Msg("Error writing file")
		}
		log.Info().Msgf("Generated example config: %s", outputFile)
	}
}
