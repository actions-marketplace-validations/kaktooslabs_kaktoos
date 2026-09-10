package main

import (
	"fmt"
	"os"

	"github.com/kaktooslabs/kaktoos/internal/scenario"
)

// loadScenarios reads scenario files then parses inline YAML strings, in that
// order. Fail-fast: the first bad file or inline string aborts the load.
func loadScenarios(paths []string, inline []string) ([]*scenario.Scenario, error) {
	scenarios := make([]*scenario.Scenario, 0, len(paths)+len(inline))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading scenario file: %w", err)
		}
		scn, err := scenario.Load(data)
		if err != nil {
			return nil, fmt.Errorf("loading scenario %s: %w", path, err)
		}
		scenarios = append(scenarios, scn)
	}
	for i, y := range inline {
		scn, err := scenario.Load([]byte(y))
		if err != nil {
			return nil, fmt.Errorf("loading inline scenario #%d: %w", i+1, err)
		}
		scenarios = append(scenarios, scn)
	}
	return scenarios, nil
}
