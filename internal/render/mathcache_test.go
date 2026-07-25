package render

// Internal test: the cache is package state, and the point of these cases
// is that it is consulted, not merely that rendering still works.

import (
	"fmt"
	"testing"
)

func resetMathCache() {
	mathCache.Lock()
	clear(mathCache.entries)
	mathCache.Unlock()
}

func mathCacheLen() int {
	mathCache.Lock()
	defer mathCache.Unlock()
	return len(mathCache.entries)
}

// TestMathCacheReusesRaster is the whole point of the cache: rasterizing
// dominates a render pass, so a repeated expression must not redo it.
func TestMathCacheReusesRaster(t *testing.T) {
	resetMathCache()
	const expr = `\frac{a+b}{c-d}`

	first, fgKey, err := renderMath(expr)
	if err != nil {
		t.Fatalf("renderMath: %v", err)
	}
	if mathCacheLen() != 1 {
		t.Fatalf("cache holds %d entries after one render, want 1", mathCacheLen())
	}

	second, fgKey2, err := renderMath(expr)
	if err != nil {
		t.Fatalf("renderMath (cached): %v", err)
	}
	// Same backing image means the raster was reused, not redrawn.
	if first != second {
		t.Error("second render produced a new raster; the cache was not consulted")
	}
	if fgKey != fgKey2 {
		t.Errorf("fgKey changed across renders: %q then %q", fgKey, fgKey2)
	}
	if mathCacheLen() != 1 {
		t.Errorf("cache grew to %d entries for one expression", mathCacheLen())
	}
}

// TestMathCacheMemoizesFailure covers the case CLAUDE.md calls out: go-latex
// cannot typeset sub- or superscripts, so those documents take the fallback
// path on every frame and must not re-attempt the raster each time.
func TestMathCacheMemoizesFailure(t *testing.T) {
	resetMathCache()
	const expr = `x_1 + y^2`

	if _, _, err := renderMath(expr); err == nil {
		t.Fatal("expected go-latex to reject sub/superscripts")
	}
	if mathCacheLen() != 1 {
		t.Fatalf("failure was not cached: %d entries", mathCacheLen())
	}
	if _, _, err := renderMath(expr); err == nil {
		t.Error("cached failure did not surface as an error")
	}
	if mathCacheLen() != 1 {
		t.Errorf("cache grew to %d entries for one failing expression", mathCacheLen())
	}
}

// TestMathCacheDistinguishesExpressions guards against a key that collides.
func TestMathCacheDistinguishesExpressions(t *testing.T) {
	resetMathCache()
	a, _, err := renderMath(`\frac{a}{b}`)
	if err != nil {
		t.Fatalf("renderMath: %v", err)
	}
	b, _, err := renderMath(`\frac{c}{d}`)
	if err != nil {
		t.Fatalf("renderMath: %v", err)
	}
	if a == b {
		t.Error("two different expressions returned the same raster")
	}
	if mathCacheLen() != 2 {
		t.Errorf("cache holds %d entries for two expressions, want 2", mathCacheLen())
	}
}

// TestMathCacheStaysBounded checks the cap: the cache resets rather than
// growing without limit on a pathological document.
func TestMathCacheStaysBounded(t *testing.T) {
	resetMathCache()
	// Failing expressions are cheap and exercise the same bookkeeping.
	for i := 0; i < maxCachedMath+5; i++ {
		renderMath(fmt.Sprintf(`x_%d`, i)) //nolint:errcheck // the failure is the point
	}
	if n := mathCacheLen(); n > maxCachedMath {
		t.Errorf("cache holds %d entries, want at most %d", n, maxCachedMath)
	}
	resetMathCache()
}
