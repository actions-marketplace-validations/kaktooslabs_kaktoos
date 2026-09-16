package discovery

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfigNames are the file names checked, in order, at the repository root.
var ConfigNames = []string{"kaktoos.yaml", "kaktoos.yml"}

// Config is the optional repository manifest that declares what services
// exist and which specs and scenarios belong to them. When absent, discovery
// falls back to convention and marks what it finds as inferred.
type Config struct {
	Services []ServiceConfig `yaml:"services"`
}

// ServiceConfig declares one service.
type ServiceConfig struct {
	Name string `yaml:"name"`
	// Paths are repo-relative globs owned by this service, e.g. "payment-service/**".
	Paths []string `yaml:"paths"`
	// OpenAPI is a repo-relative path to this service's specification.
	OpenAPI string `yaml:"openapi,omitempty"`
	// Scenarios is a repo-relative directory of Kaktoos scenario files.
	Scenarios string `yaml:"scenarios,omitempty"`
	// DependsOn names other services this one calls.
	DependsOn []string `yaml:"depends_on,omitempty"`
}

// ConfigError marks a malformed manifest, mirroring config.ConfigError so the
// CLI can exit 2 on it.
type ConfigError struct{ Message string }

func (e *ConfigError) Error() string { return "kaktoos config: " + e.Message }

// FindConfig returns the path to the repository manifest, or "" when the
// repository does not have one.
func FindConfig(root string) string {
	for _, name := range ConfigNames {
		p := filepath.Join(root, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// LoadConfig reads and validates a repository manifest.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("cannot read %s: %v", path, err)}
	}
	return LoadConfigBytes(data)
}

// LoadConfigBytes parses a manifest from memory. Unknown fields are rejected
// rather than silently ignored, matching scenario.Load's strictness — a typo
// in a manifest would otherwise silently drop a service from impact analysis.
func LoadConfigBytes(data []byte) (*Config, error) {
	var raw struct {
		Services []map[string]any `yaml:"services"`
		Rest     map[string]any   `yaml:",inline"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("not valid YAML: %v", err)}
	}
	for k := range raw.Rest {
		return nil, &ConfigError{Message: fmt.Sprintf("unknown field %q", k)}
	}
	allowed := map[string]bool{"name": true, "paths": true, "openapi": true, "scenarios": true, "depends_on": true}
	for i, svc := range raw.Services {
		for k := range svc {
			if !allowed[k] {
				return nil, &ConfigError{Message: fmt.Sprintf("services[%d]: unknown field %q", i, k)}
			}
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, &ConfigError{Message: fmt.Sprintf("not valid YAML: %v", err)}
	}
	if len(cfg.Services) == 0 {
		return nil, &ConfigError{Message: "at least one service is required"}
	}
	seen := map[string]bool{}
	for i, s := range cfg.Services {
		if s.Name == "" {
			return nil, &ConfigError{Message: fmt.Sprintf("services[%d].name is required", i)}
		}
		if seen[s.Name] {
			return nil, &ConfigError{Message: fmt.Sprintf("services[%d]: duplicate service name %q", i, s.Name)}
		}
		seen[s.Name] = true
		if len(s.Paths) == 0 {
			return nil, &ConfigError{Message: fmt.Sprintf("services[%d] (%s): at least one path glob is required", i, s.Name)}
		}
	}
	return &cfg, nil
}
