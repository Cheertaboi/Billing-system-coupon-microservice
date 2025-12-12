package concurrrencypackage concurrency

import (
	"context"
	"sync"
)

// Small reusable worker pool pattern (not required; included for re-use).
// This file provides a convenience function to fan-out tasks and collect results.

type WorkerFn func(ctx context.Context, index int)

func SimpleWorkerPool(ctx context.Context, concurrency int, tasks int, fn WorkerFn) {
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fn(ctx, idx)
		}(i)
	}
	wg.Wait()
}
