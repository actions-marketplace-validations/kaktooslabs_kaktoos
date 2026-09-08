package openapi

import (
	"github.com/getkin/kin-openapi/openapi3"
	"strings"
)

// Operation represents a single API endpoint operation (e.g., GET, POST).
type Operation struct {
	// Name is the operation ID from the OpenAPI spec (e.g., "getUserDetails").
	Name string `yaml:"operationId,omitempty"`
	// Description provides a detailed description of what the endpoint does.
	Description string `yaml:"description,omitempty"`
	// Parameters list all parameters accepted by this operation.
	Parameters []Parameter `yaml:"parameters,omitempty"`
	// RequestBody defines the expected structure and content type of the request body.
	RequestBody *openapi3.RequestBody `yaml:"requestBody,omitempty"`
	// Responses maps HTTP status codes (e.g., 200, 404) to response objects.
	Responses map[string]*openapi3.Response `yaml:"responses,omitempty"`
	// Tags provides categorization for the endpoint (e.g., "User Management").
	Tags []string `yaml:"tags,omitempty"`
}

// Parameter defines an individual query, path, or header parameter.
type Parameter struct {
	// Name is the name of the parameter.
	Name string `yaml:"name,omitempty"`
	// In indicates where the parameter is located: path, query, header, or cookie.
	In string `yaml:"in,omitempty"`
	// Description provides a detailed description of the parameter.
	Description string `yaml:"description,omitempty"`
	// Required indicates if the parameter must be present.
	Required bool `yaml:"required,omitempty"`
	// Example is an example value for the parameter.
	Example string `yaml:"example,omitempty"`
}

// OperationMap holds all discovered operations, keyed by the path segment.
// Example: {"/users": {"GET": Operation, "POST": Operation}, "/products/details": {"GET": Operation}}
type OperationMap map[string]OperationMapInner

// OperationMapInner holds all operations (GET, POST, etc.) for a specific path segment.
type OperationMapInner map[string]Operation

// cleanOperationId takes the operation ID and makes it safe for use as a map key.
// It converts the string to a canonical format, usually lowercase and hyphenated.
func cleanOperationId(id string) string {
	return strings.ToLower(strings.ReplaceAll(id, " ", "-"))
}
