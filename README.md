# Kaktoos
 
API regression testing tool for validating APIs against OpenAPI specifications.
 
## Installation
 
```bash
go install github.com/kaktooslabs/kaktoos/cmd/kaktoos@latest
```
 
Or build from source:
 
```bash
git clone https://github.com/kaktooslabs/kaktoos.git
cd kaktoos
go build ./cmd/kaktoos
```
 
## Usage
 
### Validate Configuration Files
 
```bash
kaktoos validate \
  --openapi api-spec.yml \
  --env environments/local.yml \
  --scenario scenarios/test.yml
```
 
### Run Test Scenario
 
```bash
kaktoos run \
  --openapi api-spec.yml \
  --env environments/local.yml \
  --scenario scenarios/test.yml
```
 
## Configuration Files
 
### OpenAPI Specification
 
Standard OpenAPI 3.0 specification with `operationId` for each endpoint.
 
### Environment File
 
```yaml
baseURL: http://localhost:8080
variables:
  userId: test-user-123
headers:
  Authorization: Bearer token
```
 
### Scenario File
 
```yaml
name: User Management Test
steps:
  - name: Create user
    operation: createUser
    request:
      body:
        name: Test User
    extract:
      userId: $.id
    assert:
      status: 201
      body:
        equals:
          name: Test User
  - name: Get user
    operation: getUser
    request:
      path:
        id: "{{userId}}"
    assert:
      status: 200
      body:
        equals:
          id: "{{userId}}"
          name: Test User
```
 
## Examples
 
See `examples/` directory for complete working examples.
 
## Testing
 
Run unit tests:
```bash
go test ./...
```
 
Run end-to-end tests:
```bash
go test -tags e2e ./e2e/...
```
 
## License
 
MIT
