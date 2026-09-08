package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"
)

// Environment holds the runtime configuration for a single environment.
type Environment struct {
	BaseURL   string            `yaml:"base_url"`
	Headers   map[string]string `yaml:"headers"`
	Variables map[string]string `yaml:"variables"`
}

// ConfigError wraps an error that occurs during configuration loading,
// suggesting an exit code 2 failure path.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config: %s", e.Message)
}

// Load parses an environment YAML file and validates required fields.
// Returns a ConfigError on any failure.
func Load(path string) (Environment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Environment{}, &ConfigError{
				Message: fmt.Sprintf("file not found: %s", path),
			}
		}
		return Environment{}, &ConfigError{
			Message: fmt.Sprintf("cannot read file %s: %v", path, err),
		}
	}

	var env Environment
	if err := yaml.Unmarshal(data, &env); err != nil {
		return Environment{}, &ConfigError{
			Message: fmt.Sprintf("file %s cannot be parsed as valid YAML: %s", path, err.Error()),
		}
	}

	// Validate BaseURL (required)
	if env.BaseURL == "" {
		return Environment{}, &ConfigError{
			Message: "base_url is required in the environment file",
		}
	}

	u, err := url.Parse(env.BaseURL)
	if err != nil {
		return Environment{}, &ConfigError{
			Message: fmt.Sprintf("base_url %q is not a valid absolute URL: %v", env.BaseURL, err),
		}
	}

	// Require http or https scheme and a non-empty host.
	if u.Scheme != "http" && u.Scheme != "https" {
		return Environment{}, &ConfigError{
			Message: fmt.Sprintf("base_url %q is not a valid absolute http/https URL", env.BaseURL),
		}
	}
	if u.Host == "" {
		return Environment{}, &ConfigError{
			Message: fmt.Sprintf("base_url %q has an empty host", env.BaseURL),
		}
	}

	// Initialise optional maps so callers never see nil.
	if env.Headers == nil {
		env.Headers = make(map[string]string)
	}
	if env.Variables == nil {
		env.Variables = make(map[string]string)
	}

	return env, nil
}
