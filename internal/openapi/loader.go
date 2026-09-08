package openapi

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// Load loads an OpenAPI specification from a file path and populates the OperationMap.
// It returns the OperationMap or an error if loading or validation fails.
func Load(filePath string) (OperationMap, error) {
	// 1. Load the raw OpenAPI document
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load openapi spec from %s: %w", filePath, err)
	}

	// 2. Validate the entire specification
	if err = spec.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("openapi spec validation failed: %w", err)
	}

	operationMap := make(OperationMap)

	// 3. Iterate through paths and operations to build OperationMap
	for path, pathItem := range spec.Paths.Map() {
		pathOuter := path // Capture the current path string

		// Initialize the inner map for this path
		pathInner := make(OperationMapInner)

		// Iterate over all operations (GET, POST, etc.) for the current path
		for method, pathItemOp := range pathItem.Operations() {
			op, err := buildOperationFromOpenAPI(pathOuter, method, pathItemOp)
			if err != nil {
				// Log the error but continue loading other operations if possible,
				// or return the error if strict validation is required.
				// For safety, we return the error.
				return nil, fmt.Errorf("failed to process operation %s %s: %w", method, pathOuter, err)
			}

			// Store the built operation
			pathInner[method] = op
		}

		// Store the inner map under the path key
		operationMap[path] = pathInner
	}

	return operationMap, nil
}

// buildOperationFromOpenAPI extracts required fields from kin-openapi types
// into the custom Operation struct to maintain domain structure.
func buildOperationFromOpenAPI(path, method string, op *openapi3.Operation) (Operation, error) {
	opStruct := Operation{
		Name:        op.OperationID,
		Description: op.Description,
		Tags:        op.Tags,
		Parameters:  make([]Parameter, 0, len(op.Parameters)),
		Responses:   make(map[string]*openapi3.Response),
	}

	// Process Parameters
	for _, ref := range op.Parameters {
		param := ref.Value
		if param == nil {
			continue
		}
		paramStruct := Parameter{
			Name:        param.Name,
			In:          param.In,
			Description: param.Description,
			Required:    param.Required,
		}
		if param.Example != nil {
			paramStruct.Example = fmt.Sprint(param.Example)
		}
		opStruct.Parameters = append(opStruct.Parameters, paramStruct)
	}

	// Process Responses
	if op.Responses != nil {
		for code, ref := range op.Responses.Map() {
			if ref != nil {
				opStruct.Responses[code] = ref.Value
			}
		}
	}

	return opStruct, nil
}
