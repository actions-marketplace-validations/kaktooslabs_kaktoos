// Command server is the Orders API implementation for the Kaktoos
// AI-agent verification demo.
//
// It stands in for backend code an AI coding agent just wrote. The agent read
// the ticket, not the contract, and named the money field `amount` — the
// OpenAPI document calls it `total`. Both spellings look completely reasonable
// in isolation, which is exactly why this class of mistake survives code
// review and unit tests.
//
// Set ORDERS_FIXED=1 to serve the contract-correct response instead.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const addr = "127.0.0.1:8090"

func main() {
	fixed := os.Getenv("ORDERS_FIXED") == "1"

	http.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/orders/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}

		// The response is otherwise identical: same order, same value, same
		// currency. Only the name of the money field differs.
		body := map[string]any{"id": id, "currency": "USD"}
		if fixed {
			body["total"] = 100
		} else {
			body["amount"] = 100
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	})

	state := "BROKEN (returns `amount`, contract requires `total`)"
	if fixed {
		state = "FIXED (returns `total`)"
	}
	fmt.Printf("Orders API listening on http://%s — %s\n", addr, state)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
