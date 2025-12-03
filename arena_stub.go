package skiplist

// Minimal arena shim: restores the small public API surface expected by
// other files (Arena, ArenaOption, and growth option helpers). This file
// provides a safe, minimal implementation used to satisfy builds and
// tests after the original raw-byte Arena was removed.

// NOTE: This is a lightweight shim. If you want the original raw-byte
// Arena behavior, reintroduce the full implementation separately.

type Arena struct {
	growthFactor    float64
	growthBytes     int
	growthThreshold float64
}

// ArenaOption configures an Arena.
type ArenaOption func(*Arena)

// WithGrowthFactor sets a growth factor on the provided Arena.
func WithGrowthFactor(factor float64) ArenaOption {
	return func(a *Arena) {
		if factor > 1.0 {
			a.growthFactor = factor
		}
	}
}

// WithGrowthBytes sets the fixed-bytes growth option on the provided Arena.
func WithGrowthBytes(bytes int) ArenaOption {
	return func(a *Arena) {
		if bytes > 0 {
			a.growthBytes = bytes
		}
	}
}

// WithGrowthThreshold sets a proactive growth threshold on the provided Arena.
func WithGrowthThreshold(threshold float64) ArenaOption {
	return func(a *Arena) {
		if threshold > 0.0 && threshold < 1.0 {
			a.growthThreshold = threshold
		}
	}
}

// NewArena creates a minimal Arena instance. The real allocation behavior is
// intentionally omitted; this is a shim to provide the configuration API
// used elsewhere in the codebase.
func NewArena(initialSize int, opts ...ArenaOption) *Arena {
	a := &Arena{}
	for _, opt := range opts {
		if opt != nil {
			opt(a)
		}
	}
	return a
}
