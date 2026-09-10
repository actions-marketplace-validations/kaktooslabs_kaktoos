package idempotency

import "testing"

func TestMemoryStore(t *testing.T) {
	s := NewMemoryStore()
	if v, _ := s.Get("x"); v != nil {
		t.Fatal("hit")
	}
	if e := s.Set("x", Record{Key: "x"}); e != nil {
		t.Fatal(e)
	}
	if v, _ := s.Get("x"); v == nil || v.Key != "x" {
		t.Fatal("miss")
	}
}
