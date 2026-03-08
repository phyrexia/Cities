package coordinator

import (
	"log"
	"sync"
)

// GlobalRefugeePool tracks people who left collapsing cities.
// During each heartbeat tick, attractive cities absorb from this pool.
// It simulates the reality that economic migrants go somewhere.
type GlobalRefugeePool struct {
	mu    sync.Mutex
	total int
	log   []RefugeeEvent
}

// RefugeeEvent records a pool change for audit/history.
type RefugeeEvent struct {
	Round  int
	Delta  int    // positive = added, negative = absorbed
	Reason string
}

// NewGlobalRefugeePool creates an empty pool.
func NewGlobalRefugeePool() *GlobalRefugeePool {
	return &GlobalRefugeePool{}
}

// Add puts migrants into the pool (called when a city loses population).
func (p *GlobalRefugeePool) Add(count int, round int, reason string) {
	if count <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.total += count
	p.log = append(p.log, RefugeeEvent{Round: round, Delta: count, Reason: reason})
	log.Printf("[RefugeePool] +%d (%s) → total: %d", count, reason, p.total)
}

// Available returns how many refugees are in the pool.
func (p *GlobalRefugeePool) Available() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.total
}

// Absorb removes 'count' refugees from the pool (called when a city accepts them).
// Returns how many were actually absorbed (may be less than requested).
func (p *GlobalRefugeePool) Absorb(count int, round int, cityName string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if count > p.total {
		count = p.total
	}
	if count <= 0 {
		return 0
	}
	p.total -= count
	p.log = append(p.log, RefugeeEvent{
		Round: round, Delta: -count,
		Reason: "absorbed by " + cityName,
	})
	return count
}

// Total returns the pool size (alias for Available).
func (p *GlobalRefugeePool) Total() int {
	return p.Available()
}
