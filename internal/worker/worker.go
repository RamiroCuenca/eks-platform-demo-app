// Package worker drains the Redis work queue. It is the workload KEDA scales on the
// queue's length, so each job carries a small simulated processing delay to keep a
// visible backlog under load.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/RamiroCuenca/eks-platform-demo-app/internal/cache"
	"github.com/RamiroCuenca/eks-platform-demo-app/internal/obs"
)

// Run blocks, draining the queue until ctx is cancelled.
func Run(ctx context.Context, c *cache.Cache, logger *slog.Logger) {
	logger.Info("worker started")
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopping")
			return
		default:
		}

		job, err := c.Dequeue(ctx, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				logger.Info("worker stopping")
				return
			}
			// redis.Nil (no item before the BRPOP timeout) or a transient error —
			// back off briefly and retry rather than hot-loop.
			time.Sleep(200 * time.Millisecond)
			continue
		}

		process(job)
		obs.JobsProcessed.Inc()
		logger.Info("job processed", "job", job)
	}
}

// process simulates work so a backlog accumulates faster than it drains under load.
func process(_ string) {
	time.Sleep(50 * time.Millisecond)
}
