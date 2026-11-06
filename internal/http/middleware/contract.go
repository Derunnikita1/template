package middleware

import (
	"context"
	"time"
)

type metrics interface {
	HTTPStarted(ctx context.Context, method, route string)
	HTTPFinished(ctx context.Context, method, route string, status int, duration time.Duration)
}
