package ratelimit

import (
	"context"
	"testing"
)

func TestNoLimiter(t *testing.T) {
	if e := NewLimiter(nil).Wait(context.Background(), "missing"); e != nil {
		t.Fatal(e)
	}
}
