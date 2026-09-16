package contextmodel

import "fmt"

// Entity IDs are "<kind>:<natural key>" so they are stable across runs and
// readable in JSON output. These constructors are the only place that shape
// is defined.

func FileID(path string) string     { return "file:" + path }
func ServiceID(name string) string  { return "service:" + name }
func APIID(name string) string      { return "api:" + name }
func OwnerID(name string) string    { return "owner:" + name }
func WorkflowID(name string) string { return "workflow:" + name }

// OperationID keys an OpenAPI operation by method and path, which stay stable
// even when a spec omits or renames operationId.
func OperationID(method, path string) string {
	return fmt.Sprintf("operation:%s %s", method, path)
}

// File builds a file entity.
func File(path string) Entity {
	return Entity{Kind: KindFile, ID: FileID(path), Name: path, Attrs: map[string]string{"path": path}}
}

// Service builds a service entity.
func Service(name string) Entity {
	return Entity{Kind: KindService, ID: ServiceID(name), Name: name}
}

// Operation builds an OpenAPI operation entity. operationID may be empty.
func Operation(method, path, operationID, spec string) Entity {
	attrs := map[string]string{"method": method, "path": path}
	if operationID != "" {
		attrs["operation_id"] = operationID
	}
	if spec != "" {
		attrs["spec"] = spec
	}
	return Entity{
		Kind:  KindOperation,
		ID:    OperationID(method, path),
		Name:  method + " " + path,
		Attrs: attrs,
	}
}

// Workflow builds a Kaktoos scenario entity. file is the scenario path.
func Workflow(name, file string) Entity {
	return Entity{
		Kind:  KindWorkflow,
		ID:    WorkflowID(name),
		Name:  name,
		Attrs: map[string]string{"file": file},
	}
}

// Owner builds an owner entity (a CODEOWNERS team or user, e.g. "@payments-team").
func Owner(name string) Entity {
	return Entity{Kind: KindOwner, ID: OwnerID(name), Name: name}
}
