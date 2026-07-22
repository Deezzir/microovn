package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func readPreseedConfig() (*InitValues, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat stdin: %w", err)
	}

	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, nil
	}

	var iv InitValues
	decoder := yaml.NewDecoder(os.Stdin)
	if err := decoder.Decode(&iv); err != nil {
		return nil, fmt.Errorf("failed to parse preseed YAML from stdin: %w", err)
	}

	return &iv, nil
}
