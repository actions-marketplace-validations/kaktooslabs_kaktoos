package webhook

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level webhook server configuration.
type Config struct {
	Server ServerConfig `yaml:"server"`
	Routes []Route      `yaml:"routes"`
}

// ServerConfig holds the listen address for the webhook HTTP server.
type ServerConfig struct {
	Port int    `yaml:"port"` // 1-65535; default 8080
	Host string `yaml:"host"` // default "0.0.0.0"
}

// Route maps an inbound HTTP path/method to a workflow.
type Route struct {
	Path            string            `yaml:"path"`
	Method          string            `yaml:"method"` // stored uppercase
	Workflow        WorkflowRef       `yaml:"workflow"`
	Auth            *AuthConfig       `yaml:"auth,omitempty"`
	VariableMapping map[string]string `yaml:"variable_mapping,omitempty"`
}

// WorkflowRef points to the files needed to execute a workflow.
type WorkflowRef struct {
	OpenAPIPath     string `yaml:"openapi"`
	EnvironmentPath string `yaml:"environment"`
	ScenarioPath    string `yaml:"scenario"`
}

// AuthConfig describes how a route's requests are authenticated.
type AuthConfig struct {
	Type     string `yaml:"type"` // bearer_token | hmac_sha256 | basic
	Token    string `yaml:"token,omitempty"`
	Secret   string `yaml:"secret,omitempty"`
	Header   string `yaml:"header,omitempty"` // default X-Hub-Signature-256 for hmac_sha256
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

// LoadConfig reads and validates a webhook config file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read webhook config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("webhook config %s is not valid YAML: %w", path, err)
	}

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return nil, fmt.Errorf("webhook config: server.port must be between 1 and 65535, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}

	if len(cfg.Routes) == 0 {
		return nil, fmt.Errorf("webhook config: at least one route is required")
	}

	for i := range cfg.Routes {
		r := &cfg.Routes[i]
		if r.Path == "" {
			return nil, fmt.Errorf("webhook config: routes[%d].path is required", i)
		}
		if r.Method == "" {
			return nil, fmt.Errorf("webhook config: routes[%d].method is required", i)
		}
		r.Method = strings.ToUpper(r.Method)
		if r.Workflow.OpenAPIPath == "" {
			return nil, fmt.Errorf("webhook config: routes[%d].workflow.openapi is required", i)
		}
		if r.Workflow.EnvironmentPath == "" {
			return nil, fmt.Errorf("webhook config: routes[%d].workflow.environment is required", i)
		}
		if r.Workflow.ScenarioPath == "" {
			return nil, fmt.Errorf("webhook config: routes[%d].workflow.scenario is required", i)
		}

		if r.Auth == nil {
			continue
		}
		switch r.Auth.Type {
		case "bearer_token":
			if r.Auth.Token == "" {
				return nil, fmt.Errorf("webhook config: routes[%d].auth.token is required for bearer_token", i)
			}
		case "hmac_sha256":
			if r.Auth.Secret == "" {
				return nil, fmt.Errorf("webhook config: routes[%d].auth.secret is required for hmac_sha256", i)
			}
			if r.Auth.Header == "" {
				r.Auth.Header = "X-Hub-Signature-256"
			}
		case "basic":
			if r.Auth.Username == "" || r.Auth.Password == "" {
				return nil, fmt.Errorf("webhook config: routes[%d].auth.username and password are required for basic", i)
			}
		default:
			return nil, fmt.Errorf("webhook config: routes[%d].auth.type must be one of bearer_token, hmac_sha256, basic, got %q", i, r.Auth.Type)
		}
	}

	return &cfg, nil
}
