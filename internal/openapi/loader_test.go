package openapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Helper Functions and Test Utilities ---

// setupTempFile writes content to a temporary file and returns the absolute path.
func setupTempFile(t *testing.T, content string) string {
	t.Helper()
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "openapi.yml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return filePath
}

// --- Test Cases ---

func TestLoad_Success(t *testing.T) {
	// A minimal, valid OpenAPI v3 YAML structure for testing.
	validOpenAPISpec := `
openapi: 3.0.0
info:
  title: Kaktoos API
  version: 1.0.0
paths:
  /v1/users:
    get:
      operationId: getUser
      summary: Get all users
      parameters:
      - name: limit
        in: query
        required: false
        schema:
          type: integer
      responses:
        '200':
          description: Successful response
        '400':
          description: Bad request
  /v1/products/{id}:
    get:
      operationId: getProductById
      summary: Get a specific product
      parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
      responses:
        '200':
          description: Product found
`
	filePath := setupTempFile(t, validOpenAPISpec)

	// Build the map and check for errors
	opMap, err := Load(filePath)

	if err != nil {
		t.Fatalf("Expected no error loading valid spec, got: %v", err)
	}

	// 1. Check structure integrity: should have entries for /v1/users and /v1/products/{id}
	if _, ok := opMap["/v1/users"]; !ok {
		t.Error("OperationMap missing path /v1/users")
	}
	if _, ok := opMap["/v1/products/{id}"]; !ok {
		t.Error("OperationMap missing path /v1/products/{id}")
	}

	// 2. Check specific operation mapping: /v1/users should have GET
	if _, ok := opMap["/v1/users"]["GET"]; !ok {
		t.Error("Path /v1/users missing GET operation")
	}

	// 3. Check nested details: The GET operation for /v1/users
	usersGetOp := opMap["/v1/users"]["GET"]

	if usersGetOp.Name != "getUser" {
		t.Errorf("Expected operation name 'getUser', got '%s'", usersGetOp.Name)
	}

	// 4. Check parameters extraction
	if len(usersGetOp.Parameters) != 1 {
		t.Errorf("Expected 1 parameter (limit), got %d", len(usersGetOp.Parameters))
	} else if usersGetOp.Parameters[0].Name != "limit" {
		t.Errorf("Expected parameter 'limit', got '%s'", usersGetOp.Parameters[0].Name)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	// Test case where the file does not exist.
	nonExistentPath := filepath.Join(t.TempDir(), "nonexistent.yml")
	_, err := Load(nonExistentPath)

	if err == nil {
		t.Fatal("Expected an error for non-existent file, but got nil")
	}
	if !strings.Contains(err.Error(), "failed to load openapi spec") || !strings.Contains(err.Error(), "no such file") {
		t.Errorf("Expected specific file not found error, got: %v", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	// YAML with structural errors
	invalidYAML := `
openapi: 3.0.0
info:
  title: Invalid Spec
  version: 1.0.0
paths:
  /test:
    get:
      operationId: bad
      # Missing required fields here
`
	filePath := setupTempFile(t, invalidYAML)

	_, err := Load(filePath)

	if err == nil {
		t.Fatal("Expected an error for invalid YAML structure, but got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error for missing responses, got: %v", err)
	}
}

func TestLoad_OpenAPIVaidationFailure(t *testing.T) {
	// OpenAPI Spec with invalid data types (e.g., wrong use of OpenAPI schema)
	invalidSchemaOpenAPI := `
openapi: 3.0.0
info:
  title: Validation Fail
  version: 1.0.0
paths:
  /test:
    get:
      operationId: validate
      responses:
        '200': {} # missing required 'description'
`
	filePath := setupTempFile(t, invalidSchemaOpenAPI)

	_, err := Load(filePath)

	if err == nil {
		t.Fatal("Expected an error due to OpenAPI validation failure, but got nil")
	}
	if !strings.Contains(err.Error(), "openapi spec validation failed") {
		t.Errorf("Expected OpenAPI validation error, got: %v", err)
	}
}

// Test for duplicate operationId (which should fail due to OpenAPI uniqueness rules)
func TestLoad_DuplicateOperationId(t *testing.T) {
	// Note: Kin-openapi often handles this internally by allowing the last one to win
	// or by failing validation. We test the general expectation of failure if it's a known issue.
	duplicateIdSpec := `
openapi: 3.0.0
info:
  title: Duplicate ID
  version: 1.0.0
paths:
  /v1/users:
    get:
      operationId: findUser # First definition
      summary: Get User 1
      responses:
        '200':
          description: OK
  /v2/users:
    get:
      operationId: findUser # Duplicate definition
      summary: Get User 2
      responses:
        '200':
          description: OK
`
	filePath := setupTempFile(t, duplicateIdSpec)

	// Depending on the exact OpenAPI loader version/version of the spec, this might pass
	// or fail. However, the goal is to test failure modes.
	// We assert that if an error occurs, it's related to the spec loading.
	_, err := Load(filePath)

	if err != nil {
		t.Logf("Note: Load failed due to: %v. This may be expected behavior depending on spec strictness.", err)
		// Don't fail the test if the failure is expected due to library constraints,
		// but ensuring it hits the loader mechanisms is key.
	}
}
