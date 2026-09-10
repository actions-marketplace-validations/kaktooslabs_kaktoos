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
	BaseURL    string                    `yaml:"base_url"`
	Headers    map[string]string         `yaml:"headers"`
	Variables  map[string]string         `yaml:"variables"`
	RateLimits map[string]RateLimitEntry `yaml:"rate_limits,omitempty"`
}
type RateLimitEntry struct {
	RequestsPerSecond *int `yaml:"requests_per_second,omitempty"`
	RequestsPerMinute *int `yaml:"requests_per_minute,omitempty"`
}

// ConfigError wraps an error that occurs during configuration loading,
// suggesting an exit code 2 failure path.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config: %s", e.Message)
}

// Load reads an environment YAML file and parses it via LoadBytes.
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
	return LoadBytes(data)
}

// LoadBytes parses environment YAML from memory and validates required
// fields. Used by Load, and directly by callers with inline YAML (e.g. MCP).
func LoadBytes(data []byte) (Environment, error) {
	var env Environment
	if err := yaml.Unmarshal(data, &env); err != nil {
		return Environment{}, &ConfigError{
			Message: fmt.Sprintf("cannot be parsed as valid YAML: %s", err.Error()),
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
	for key, limit := range env.RateLimits {
		if limit.RequestsPerSecond != nil && limit.RequestsPerMinute != nil {
			return Environment{}, &ConfigError{Message: fmt.Sprintf("rate_limits.%s: cannot specify both requests_per_second and requests_per_minute", key)}
		}
		if limit.RequestsPerSecond == nil && limit.RequestsPerMinute == nil {
			return Environment{}, &ConfigError{Message: fmt.Sprintf("rate_limits.%s: must specify either requests_per_second or requests_per_minute", key)}
		}
		if (limit.RequestsPerSecond != nil && *limit.RequestsPerSecond <= 0) || (limit.RequestsPerMinute != nil && *limit.RequestsPerMinute <= 0) {
			return Environment{}, &ConfigError{Message: fmt.Sprintf("rate_limits.%s: rate limit value must be greater than zero", key)}
		}
	}

	return env, nil
}
