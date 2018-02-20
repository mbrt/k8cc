package service

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc/pkg/data"
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

func (mw loggingMiddleware) LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (r Lease, err error) {
	defer func(begin time.Time) {
		lerr := mw.logger.Log("method", "LeaseDistcc", "user", u, "tag", t, "took", time.Since(begin), "err", err)
		if err == nil {
			err = lerr
		}
	}(time.Now())
	return mw.next.LeaseDistcc(ctx, u, t)
}

func (mw loggingMiddleware) DeleteDistcc(ctx context.Context, u data.User, t data.Tag) (err error) {
	defer func(begin time.Time) {
		lerr := mw.logger.Log("method", "DeleteDistcc", "user", u, "tag", t, "took", time.Since(begin), "err", err)
		if err == nil {
			err = lerr
		}
	}(time.Now())
	return mw.next.DeleteDistcc(ctx, u, t)
}

func (mw loggingMiddleware) LeaseClient(ctx context.Context, u data.User, t data.Tag) (r Lease, err error) {
	defer func(begin time.Time) {
		lerr := mw.logger.Log("method", "LeaseClient", "user", u, "tag", t, "took", time.Since(begin), "err", err)
		if err == nil {
			err = lerr
		}
	}(time.Now())
	return mw.next.LeaseClient(ctx, u, t)
}
