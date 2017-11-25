package api

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
)

// Middleware describes a service (as opposed to endpoint) middleware.
type Middleware func(Service) Service

// LoggingMiddleware implements a service with logging
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) LeaseUser(ctx context.Context, user, tag string) (r Lease, err error) {
	defer func(begin time.Time) {
		lerr := mw.logger.Log("method", "LeaseUser", "user", user, "tag", tag, "took", time.Since(begin), "err", err)
		if err != nil {
			err = lerr
		}
	}(time.Now())
	return mw.next.LeaseUser(ctx, user, tag)
}
