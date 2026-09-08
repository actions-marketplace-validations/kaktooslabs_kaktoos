#!/usr/bin/env bash

set -u

KAKTOOS_BIN="${KAKTOOS_BIN:-./kaktoos}"
PORT="${KAKTOOS_TEST_PORT:-18080}"
BASE_URL="http://127.0.0.1:${PORT}"

TMP_DIR="$(mktemp -d)"
SERVER_PID=""

PASS=0
FAIL=0

cleanup() {
  if [ -n "${SERVER_PID}" ]; then
    kill "${SERVER_PID}" >/dev/null 2>&1 || true
    wait "${SERVER_PID}" 2>/dev/null || true
  fi

  rm -rf "${TMP_DIR}"
}

trap cleanup EXIT

pass() {
  echo "✅ PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "❌ FAIL: $1"
  FAIL=$((FAIL + 1))
}

echo
echo "========================================"
echo " Kaktoos MVP Functional Test"
echo "========================================"
echo

if [ ! -x "${KAKTOOS_BIN}" ]; then
  echo "Kaktoos binary not found: ${KAKTOOS_BIN}"
  echo
  echo "Build it first:"
  echo "  go build -o kaktoos ./cmd/kaktoos"
  exit 1
fi

# ------------------------------------------------------------
# Mock API Server
# ------------------------------------------------------------

cat > "${TMP_DIR}/server.py" <<'PY'
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse, parse_qs
import json
import sys

PORT = int(sys.argv[1])

class Handler(BaseHTTPRequestHandler):

    def send_json(self, status, body):
        payload = json.dumps(body).encode()

        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()

        self.wfile.write(payload)

    def do_POST(self):
        if self.path == "/users":
            content_length = int(self.headers.get("Content-Length", "0"))
            raw = self.rfile.read(content_length)

            try:
                body = json.loads(raw)
            except Exception:
                self.send_json(400, {
                    "error": "invalid json"
                })
                return

            if body.get("name") != "Test User":
                self.send_json(400, {
                    "error": "invalid name"
                })
                return

            if self.headers.get("X-Test-Client") != "kaktoos":
                self.send_json(400, {
                    "error": "missing X-Test-Client header"
                })
                return

            self.send_json(201, {
                "id": "user-123",
                "name": body["name"],
                "status": "ACTIVE"
            })
            return

        self.send_json(404, {
            "error": "not found"
        })

    def do_GET(self):
        parsed = urlparse(self.path)

        if parsed.path == "/users/user-123":
            query = parse_qs(parsed.query)

            if query.get("include") != ["profile"]:
                self.send_json(400, {
                    "error": "missing include=profile"
                })
                return

            self.send_json(200, {
                "id": "user-123",
                "name": "Test User",
                "status": "ACTIVE",
                "profile": {
                    "type": "individual"
                }
            })
            return

        self.send_json(404, {
            "error": "not found"
        })

    def log_message(self, format, *args):
        return


server = HTTPServer(("127.0.0.1", PORT), Handler)
server.serve_forever()
PY

python3 "${TMP_DIR}/server.py" "${PORT}" &
SERVER_PID=$!

sleep 1

if ! kill -0 "${SERVER_PID}" >/dev/null 2>&1; then
  echo "Failed to start mock API server."
  exit 1
fi

echo "Mock API running at ${BASE_URL}"
echo

# ------------------------------------------------------------
# OpenAPI
# ------------------------------------------------------------

cat > "${TMP_DIR}/openapi.yml" <<'YAML'
openapi: 3.0.3

info:
  title: Kaktoos Functional Test API
  version: 1.0.0

paths:

  /users:
    post:
      operationId: createUser

      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - name
              properties:
                name:
                  type: string

      responses:
        "201":
          description: User created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"

  /users/{id}:
    get:
      operationId: getUser

      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string

        - name: include
          in: query
          required: false
          schema:
            type: string

      responses:
        "200":
          description: User returned
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"

components:
  schemas:
    User:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
        status:
          type: string
YAML

# ------------------------------------------------------------
# Environment
# ------------------------------------------------------------

cat > "${TMP_DIR}/environment.yml" <<YAML
name: functional-test

base_url: ${BASE_URL}

headers:
  Content-Type: application/json
  X-Test-Client: kaktoos

variables:
  includeType: profile
YAML

# ------------------------------------------------------------
# Valid Scenario
# ------------------------------------------------------------

cat > "${TMP_DIR}/scenario.yml" <<'YAML'
name: Create and Verify User

steps:

  - name: Create User
    operation: createUser

    request:
      body: |
        {
          "name": "Test User"
        }

    extract:
      userId: $.id

    assert:
      status: 201

      body:
        - path: $.id
          exists: true

        - path: $.name
          equals: "Test User"

        - path: $.status
          equals: "ACTIVE"

  - name: Get Created User
    operation: getUser

    request:
      path:
        id: "{{userId}}"

      query:
        include: "{{includeType}}"

    assert:
      status: 200

      body:
        - path: $.id
          equals: "{{userId}}"

        - path: $.name
          equals: "Test User"

        - path: $.status
          equals: "ACTIVE"

        - path: $.profile.type
          equals: "individual"

        - path: $.does_not_exist
          not_exists: true
YAML

echo "----------------------------------------"
echo "TEST 1: Validate valid files"
echo "----------------------------------------"

"${KAKTOOS_BIN}" validate \
  --openapi "${TMP_DIR}/openapi.yml" \
  --env "${TMP_DIR}/environment.yml" \
  --scenario "${TMP_DIR}/scenario.yml"

RESULT=$?

if [ "${RESULT}" -eq 0 ]; then
  pass "Valid OpenAPI + environment + scenario"
else
  fail "Valid files should pass validation (exit=${RESULT})"
fi

echo
echo "----------------------------------------"
echo "TEST 2: Run complete scenario"
echo "----------------------------------------"

"${KAKTOOS_BIN}" run \
  --openapi "${TMP_DIR}/openapi.yml" \
  --env "${TMP_DIR}/environment.yml" \
  --scenario "${TMP_DIR}/scenario.yml"

RESULT=$?

if [ "${RESULT}" -eq 0 ]; then
  pass "Scenario execution"
else
  fail "Scenario execution should succeed (exit=${RESULT})"
fi

# ------------------------------------------------------------
# Assertion Failure
# ------------------------------------------------------------

cat > "${TMP_DIR}/assertion-failure.yml" <<'YAML'
name: Assertion Failure

steps:

  - name: Create User
    operation: createUser

    request:
      body: |
        {
          "name": "Test User"
        }

    assert:
      status: 201

      body:
        - path: $.status
          equals: "DELETED"
YAML

echo
echo "----------------------------------------"
echo "TEST 3: Assertion failure"
echo "----------------------------------------"

"${KAKTOOS_BIN}" run \
  --openapi "${TMP_DIR}/openapi.yml" \
  --env "${TMP_DIR}/environment.yml" \
  --scenario "${TMP_DIR}/assertion-failure.yml"

RESULT=$?

if [ "${RESULT}" -eq 1 ]; then
  pass "Assertion failure returns exit code 1"
else
  fail "Expected exit code 1, got ${RESULT}"
fi

# ------------------------------------------------------------
# Invalid Environment
# ------------------------------------------------------------

cat > "${TMP_DIR}/invalid-environment.yml" <<'YAML'
name: invalid

headers:
  Content-Type: application/json
YAML

echo
echo "----------------------------------------"
echo "TEST 4: Invalid environment"
echo "----------------------------------------"

"${KAKTOOS_BIN}" validate \
  --openapi "${TMP_DIR}/openapi.yml" \
  --env "${TMP_DIR}/invalid-environment.yml" \
  --scenario "${TMP_DIR}/scenario.yml"

RESULT=$?

if [ "${RESULT}" -ne 0 ]; then
  pass "Missing base_url rejected"
else
  fail "Missing base_url should fail"
fi

# ------------------------------------------------------------
# Unknown OpenAPI Operation
# ------------------------------------------------------------

cat > "${TMP_DIR}/unknown-operation.yml" <<'YAML'
name: Unknown Operation

steps:
  - name: Call missing operation
    operation: operationDoesNotExist

    assert:
      status: 200
YAML

echo
echo "----------------------------------------"
echo "TEST 5: Unknown OpenAPI operation"
echo "----------------------------------------"

"${KAKTOOS_BIN}" run \
  --openapi "${TMP_DIR}/openapi.yml" \
  --env "${TMP_DIR}/environment.yml" \
  --scenario "${TMP_DIR}/unknown-operation.yml"

RESULT=$?

if [ "${RESULT}" -ne 0 ]; then
  pass "Unknown operation rejected"
else
  fail "Unknown operation should fail"
fi

# ------------------------------------------------------------
# Invalid Scenario Schema
# ------------------------------------------------------------

cat > "${TMP_DIR}/invalid-scenario.yml" <<'YAML'
name: Invalid Scenario

steps:

  - name: Invalid body
    operation: createUser

    request:
      body:
        name: Test User
YAML

echo
echo "----------------------------------------"
echo "TEST 6: Invalid scenario schema"
echo "----------------------------------------"

"${KAKTOOS_BIN}" validate \
  --openapi "${TMP_DIR}/openapi.yml" \
  --env "${TMP_DIR}/environment.yml" \
  --scenario "${TMP_DIR}/invalid-scenario.yml"

RESULT=$?

if [ "${RESULT}" -ne 0 ]; then
  pass "Invalid scenario schema rejected"
else
  fail "Invalid scenario schema should fail"
fi

# ------------------------------------------------------------
# Summary
# ------------------------------------------------------------

echo
echo "========================================"
echo " Kaktoos Functional Test Summary"
echo "========================================"
echo
echo "Passed : ${PASS}"
echo "Failed : ${FAIL}"
echo

if [ "${FAIL}" -eq 0 ]; then
  echo "🎉 ALL KAKTOOS FUNCTIONAL TESTS PASSED"
  exit 0
else
  echo "❌ KAKTOOS FUNCTIONAL TESTS FAILED"
  exit 1
fi