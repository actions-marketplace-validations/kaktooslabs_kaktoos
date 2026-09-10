package webhook

import "testing"

func TestExtractVariables_PathAndBody(t *testing.T) {
	mapping := map[string]string{
		"orderId": "path.id",
		"name":    "body.customer.name",
	}
	pathParams := map[string]string{"id": "123"}
	body := map[string]interface{}{
		"customer": map[string]interface{}{"name": "Alice"},
	}

	vars, err := ExtractVariables(mapping, pathParams, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars["orderId"] != "123" || vars["name"] != "Alice" {
		t.Fatalf("unexpected extraction: %+v", vars)
	}
}

func TestExtractVariables_MissingPathParam(t *testing.T) {
	_, err := ExtractVariables(map[string]string{"x": "path.missing"}, map[string]string{}, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing path param")
	}
}

func TestExtractVariables_MissingBodyField(t *testing.T) {
	_, err := ExtractVariables(map[string]string{"x": "body.missing"}, map[string]string{}, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing body field")
	}
}

func TestExtractVariables_UnsupportedSource(t *testing.T) {
	_, err := ExtractVariables(map[string]string{"x": "query.foo"}, map[string]string{}, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unsupported source prefix")
	}
}

func TestExtractVariables_Empty(t *testing.T) {
	vars, err := ExtractVariables(nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vars) != 0 {
		t.Fatalf("expected empty map, got %+v", vars)
	}
}
