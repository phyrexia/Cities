package coordinator

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cities/game/internal/ai"
	"github.com/cities/game/internal/city"
	"github.com/cities/game/internal/trade"
	"github.com/google/uuid"
)

// HeartbeatInterval is the time between heartbeats.
// In production: 10 minutes. Set via env var HEARTBEAT_INTERVAL for testing.
const DefaultHeartbeatInterval = 10 * time.Minute

// Heartbeat represents one game tick and the proposals sent to a city.
type Heartbeat struct {
	ID        string
	CityID    string
	Round     int
	Proposals []ai.Initiative
	CreatedAt time.Time
	Deadline  time.Time
}

// Decision records a mayor's choice for a heartbeat.
type Decision struct {
	HeartbeatID  string
	CityID       string
	PlayerID     string
	InitiativeID string
	DecidedAt    time.Time
}

// HeartbeatResult contains what happened to a city after a heartbeat.
type HeartbeatResult struct {
	CityID      string
	City        *city.City
	Events      []city.PopulationEvent
	InitiativeID string
}

// Ticker manages the game loop and heartbeat cycle.
type Ticker struct {
	interval    time.Duration
	registry    *Registry
	generator   *ai.Generator
	tradeEngine *trade.Engine
	broadcaster *Broadcaster

	mu         sync.Mutex
	round      int
	heartbeats map[string]*Heartbeat // heartbeatID → Heartbeat
	decisions  map[string]*Decision  // cityID → latest decision
	popEngine    city.PopulationEngine
	ecoEngine    city.EconomyEngine
	refugeePool  *GlobalRefugeePool

	stopCh chan struct{}
}

// NewTicker creates a new heartbeat ticker.
func NewTicker(interval time.Duration, reg *Registry, gen *ai.Generator, te *trade.Engine, bc *Broadcaster) *Ticker {
	return &Ticker{
		interval:    interval,
		registry:    reg,
		generator:   gen,
		tradeEngine: te,
		broadcaster: bc,
		heartbeats:  make(map[string]*Heartbeat),
		decisions:   make(map[string]*Decision),
		refugeePool: NewGlobalRefugeePool(),
		stopCh:      make(chan struct{}),
	}
}

// Start begins the heartbeat loop in a background goroutine.
func (t *Ticker) Start(ctx context.Context) {
	go t.loop(ctx)
	log.Printf("[Heartbeat] Started — interval: %s", t.interval)
}

// Stop signals the ticker to stop.
func (t *Ticker) Stop() {
	close(t.stopCh)
}

// CurrentRound returns the current game round.
func (t *Ticker) CurrentRound() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.round
}

// NextHeartbeatAt returns when the next heartbeat will fire.
func (t *Ticker) NextHeartbeatAt() time.Time {
	return time.Now().Add(t.interval) // approximate
}

// RecordDecision stores a mayor's decision for the current heartbeat.
func (t *Ticker) RecordDecision(d Decision) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Validate the heartbeat exists
	hbID := d.HeartbeatID
	hb, ok := t.heartbeats[hbID]
	if !ok {
		return fmt.Errorf("heartbeat %q not found", hbID)
	}

	// Check deadline
	if time.Now().After(hb.Deadline) {
		return fmt.Errorf("heartbeat %q has expired", hbID)
	}

	// Validate initiative exists in proposals
	found := false
	for _, prop := range hb.Proposals {
		if prop.ID == d.InitiativeID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("initiative %q not found in heartbeat proposals", d.InitiativeID)
	}

	t.decisions[d.CityID] = &d
	log.Printf("[Heartbeat] Decision recorded: city=%s initiative=%s", d.CityID, d.InitiativeID)
	return nil
}

// TriggerManual forces an immediate heartbeat (for testing/admin).
func (t *Ticker) TriggerManual(ctx context.Context) {
	t.runHeartbeat(ctx)
}

func (t *Ticker) loop(ctx context.Context) {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.runHeartbeat(ctx)
		}
	}
}

func (t *Ticker) runHeartbeat(ctx context.Context) {
	t.mu.Lock()
	t.round++
	round := t.round
	t.mu.Unlock()

	log.Printf("[Heartbeat] Round %d starting", round)

	cities := t.registry.AllCities()
	if len(cities) == 0 {
		log.Println("[Heartbeat] No cities yet — skipping")
		return
	}

	// Phase 1: Settle pending trades
	citiesMap := t.registry.CitiesMap()
	tradeResults := t.tradeEngine.SettleAll(citiesMap)
	if len(tradeResults) > 0 {
		log.Printf("[Heartbeat] Settled %d trade orders", len(tradeResults))
	}

	// Phase 2: Process each city
	var results []HeartbeatResult
	for _, c := range cities {
		result := t.processCity(ctx, c, cities, round)
		results = append(results, result)
	}

	// Phase 3: Broadcast results
	for _, result := range results {
		t.broadcaster.BroadcastCityUpdate(result)
	}

	// Phase 4: Generate new heartbeats for next round
	for _, c := range cities {
		t.generateHeartbeat(ctx, c, cities, round)
	}

	// Phase 5: World event broadcast
	t.broadcaster.BroadcastWorldEvent(WorldEvent{
		Type:        "HEARTBEAT_COMPLETE",
		Description: fmt.Sprintf("Round %d complete — %d cities active", round, len(cities)),
	})

	// Clear old decisions
	t.mu.Lock()
	t.decisions = make(map[string]*Decision)
	t.mu.Unlock()

	log.Printf("[Heartbeat] Round %d complete", round)
}

func (t *Ticker) processCity(ctx context.Context, c *city.City, allCities []*city.City, round int) HeartbeatResult {
	result := HeartbeatResult{CityID: c.ID}

	// Apply pending decision from previous heartbeat
	var appliedInitiativeID string
	t.mu.Lock()
	dec, hasDecision := t.decisions[c.ID]
	t.mu.Unlock()

	if hasDecision {
		// Find the initiative
		t.mu.Lock()
		for _, hb := range t.heartbeats {
			if hb.CityID == c.ID {
				for _, prop := range hb.Proposals {
					if prop.ID == dec.InitiativeID {
						_ = t.registry.UpdateCity(c.ID, func(city *city.City) {
							city.ApplyInitiativeEffects(prop.Effects, round)
						})
						appliedInitiativeID = prop.ID
						break
					}
				}
			}
		}
		t.mu.Unlock()
		log.Printf("[Heartbeat] Applied initiative %s to city %s", appliedInitiativeID, c.Name)
	}

	// Skip tick if city is in vacation mode
	currentCity, _ := t.registry.GetCity(c.ID)
	if currentCity != nil && currentCity.IsInVacationMode() {
		log.Printf("[Heartbeat] City %s is in vacation mode — skipping tick", c.Name)
		result.City = currentCity
		return result
	}

	// Economy tick
	_ = t.registry.UpdateCity(c.ID, func(city *city.City) {
		t.ecoEngine.Tick(city)
		city.UpdateStats()
		city.UpdatePollution()
		city.ExpirePolicies(round)
		city.TrackPeakPopulation()
		city.Round = round
	})

	// Population simulation with global refugee pool
	var events []city.PopulationEvent
	var refugeesAbsorbed int
	available := t.refugeePool.Available()
	_ = t.registry.UpdateCity(c.ID, func(cty *city.City) {
		perCityShare := available / max(len(t.registry.AllCities()), 1)
		var absorbed int
		events, absorbed = t.popEngine.Simulate(cty, round, perCityShare)
		refugeesAbsorbed = absorbed
		// Count emigrants from events and add to refugee pool
		for _, ev := range events {
			if ev.Type == "LEFT" {
				t.refugeePool.Add(ev.Count, round, "emigrated from "+cty.Name)
			}
		}
	})
	if refugeesAbsorbed > 0 {
		t.refugeePool.Absorb(refugeesAbsorbed, round, c.Name)
	}

	// Check city collapse
	collapsed := false
	_ = t.registry.UpdateCity(c.ID, func(cty *city.City) {
		collapsed = cty.CheckCollapse(round)
	})
	if collapsed {
		log.Printf("[Heartbeat] City %s has COLLAPSED — entering ruins", c.Name)
		t.broadcaster.BroadcastWorldEvent(WorldEvent{
			Type:        "CITY_COLLAPSED",
			Description: c.Name + " has collapsed and become ruins.",
			CityID:      c.ID,
			CityName:    c.Name,
		})
	}

	// Get updated city state
	updated, _ := t.registry.GetCity(c.ID)
	result.City = updated
	result.Events = events
	result.InitiativeID = appliedInitiativeID

	return result
}

func max(a, b int) int {
	if a > b { return a }
	return b
}

func (t *Ticker) generateHeartbeat(ctx context.Context, c *city.City, allCities []*city.City, round int) {
	proposals, err := t.generator.GenerateForCity(ctx, c, allCities, round)
	if err != nil {
		log.Printf("[AI] Fallback for city %s: %v", c.Name, err)
		proposals = ai.FallbackInitiatives(c)
	}

	hb := &Heartbeat{
		ID:        uuid.NewString(),
		CityID:    c.ID,
		Round:     round,
		Proposals: proposals,
		CreatedAt: time.Now(),
		Deadline:  time.Now().Add(t.interval),
	}

	t.mu.Lock()
	t.heartbeats[hb.ID] = hb
	t.mu.Unlock()

	t.broadcaster.BroadcastHeartbeat(hb)
	log.Printf("[Heartbeat] Sent %d proposals to city %s", len(proposals), c.Name)
}

// WorldEvent is a global game event broadcast to all clients.
type WorldEvent struct {
	Type        string
	Description string
	CityID      string
	CityName    string
}
