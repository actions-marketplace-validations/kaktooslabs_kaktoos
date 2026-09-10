package schema

import (
	"testing"
	"time"
)

const budget = 50 * time.Millisecond

// TestValidateUnderBudget is a sanity check, not a performance guarantee:
// validation is pure in-memory work and should never dominate a step.
func TestValidateUnderBudget(t *testing.T) {
	op := loadGetUser(t)
	body := []byte(`{"id":"1","name":"n","age":3}`)
	start := time.Now()
	Validate(op, 200, "application/json", body)
	if d := time.Since(start); d > budget {
		t.Fatalf("Validate took %v, over the %v sanity budget", d, budget)
	}
}

func BenchmarkValidate(b *testing.B) {
	op := loadGetUser(b)
	body := []byte(`{"id":"1","name":"n","age":3}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Validate(op, 200, "application/json", body)
	}
}
