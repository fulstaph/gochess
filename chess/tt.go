package chess

// Transposition table for caching search results across branches.

// ttFlag classifies the bound type of a transposition table entry.
type ttFlag int

const (
	ttExact ttFlag = iota // exact score
	ttAlpha               // upper bound (failed low)
	ttBeta                // lower bound (failed high)
)

// ttEntry is a single transposition table entry.
type ttEntry struct {
	hash  uint64
	depth int
	score int
	flag  ttFlag
	best  Move // best move found at this position
}

// transTable is a fixed-size hash table for storing search results.
type transTable struct {
	entries []ttEntry
	mask    uint64
}

// newTransTable creates a transposition table with 2^logSize entries.
func newTransTable(logSize int) *transTable {
	size := uint64(1) << logSize
	return &transTable{
		entries: make([]ttEntry, size),
		mask:    size - 1,
	}
}

// probe looks up a position. Returns the entry and true if a matching entry exists.
func (tt *transTable) probe(hash uint64) (ttEntry, bool) {
	e := tt.entries[hash&tt.mask]
	if e.hash == hash {
		return e, true
	}
	return ttEntry{}, false
}

// store writes an entry into the table (always-replace strategy).
func (tt *transTable) store(hash uint64, depth, score int, flag ttFlag, best Move) {
	tt.entries[hash&tt.mask] = ttEntry{
		hash:  hash,
		depth: depth,
		score: score,
		flag:  flag,
		best:  best,
	}
}
