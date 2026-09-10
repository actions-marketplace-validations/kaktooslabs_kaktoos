package ratelimit

import (
	"context"
	"github.com/kaktooslabs/kaktoos/internal/config"
	"golang.org/x/time/rate"
	"sync"
)

type Limiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
}

func NewLimiter(limits map[string]config.RateLimitEntry) *Limiter {
	l := &Limiter{limiters: map[string]*rate.Limiter{}}
	for k, v := range limits {
		if v.RequestsPerSecond != nil {
			l.limiters[k] = rate.NewLimiter(rate.Limit(*v.RequestsPerSecond), 1)
		} else if v.RequestsPerMinute != nil {
			l.limiters[k] = rate.NewLimiter(rate.Limit(float64(*v.RequestsPerMinute)/60), 1)
		}
	}
	return l
}
func (l *Limiter) Wait(ctx context.Context, op string) error {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	x := l.limiters[op]
	l.mu.RUnlock()
	if x == nil {
		return nil
	}
	return x.Wait(ctx)
}
