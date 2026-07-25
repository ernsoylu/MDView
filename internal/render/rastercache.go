package render

import (
	"image"
	"sync"
)

// rasterResult is one memoized rasterization outcome.
type rasterResult struct {
	m   image.Image
	err error
}

// rasterCache memoizes an expensive rasterization keyed by its input.
//
// Both things mdv rasterizes — LaTeX and Mermaid — are pure functions of
// their source, and both cost far more than the rest of a render pass
// (milliseconds for LaTeX, seconds for Mermaid) while reflow() re-renders
// on every resize. Failures are memoized too: the fallback they select is
// cheaper than re-attempting, and go-latex rejects whole categories of
// input that a document will keep asking about.
//
// Past limit the cache resets, degrading to the uncached behaviour rather
// than growing without bound.
//
// The mutex is not for the UI, which renders on one goroutine, but for
// tests and fuzzing, which do not. It is deliberately not held across the
// computation: two goroutines racing on the same key both do the work,
// which is wasteful but correct, where blocking on a multi-second Mermaid
// render would not be.
type rasterCache struct {
	mu      sync.Mutex
	entries map[string]rasterResult
	limit   int
}

func newRasterCache(limit int) *rasterCache {
	return &rasterCache{entries: make(map[string]rasterResult), limit: limit}
}

func (c *rasterCache) do(key string, compute func() (image.Image, error)) (image.Image, error) {
	c.mu.Lock()
	got, hit := c.entries[key]
	c.mu.Unlock()
	if hit {
		return got.m, got.err
	}

	m, err := compute()

	c.mu.Lock()
	if len(c.entries) >= c.limit {
		clear(c.entries)
	}
	c.entries[key] = rasterResult{m: m, err: err}
	c.mu.Unlock()
	return m, err
}

func (c *rasterCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

func (c *rasterCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.entries)
}
