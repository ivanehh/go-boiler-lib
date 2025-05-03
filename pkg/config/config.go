package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config[B any] struct {
	// sourced from a yaml configuration
	Base B
	// sources from a .env file
	// Environment E
	// sources from cmd flags
	Flags map[string]any
}

type ConfigOpt[B any, E any] func(*Config[B])

func NewConfig[B any](path string) (*Config[B], error) {
	base := new(B)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	dec := yaml.NewDecoder(f)
	if err = dec.Decode(base); err != nil {
		return nil, err
	}

	config := new(Config[B])
	config.Base = *base
	return config, nil
}
