package openapi

// FindOperation resolves an operationId to its operation, path and HTTP method.
// It is the single lookup used by both the execution engine and the MCP
// adapter, so neither can drift from the other.
func FindOperation(ops OperationMap, name string) (op *Operation, path, method string, found bool) {
	for p, opMap := range ops {
		for m, o := range opMap {
			if o.Name == name {
				found := o
				return &found, p, m, true
			}
		}
	}
	return nil, "", "", false
}
