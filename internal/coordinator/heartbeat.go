package coordinator

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
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
	ID        string          `json:"id"`
	CityID    string          `json:"city_id"`
	Round     int             `json:"round"`
	Proposals []ai.Initiative `json:"proposals"`
	CreatedAt time.Time       `json:"created_at"`
	Deadline  time.Time       `json:"deadline"`
}

// Decision records a mayor's choice for a heartbeat.
type Decision struct {
	HeartbeatID  string    `json:"heartbeat_id"`
	CityID       string    `json:"city_id"`
	PlayerID     string    `json:"player_id"`
	InitiativeID string    `json:"initiative_id"`
	DecidedAt    time.Time `json:"decided_at"`
}

// HeartbeatResult contains what happened to a city after a heartbeat.
type HeartbeatResult struct {
	CityID       string                 `json:"city_id"`
	City         *city.City             `json:"city"`
	Events       []city.PopulationEvent `json:"events"`
	InitiativeID string                 `json:"initiative_id"`
}

// Ticker manages the game loop and heartbeat cycle.
type Ticker struct {
	interval    time.Duration
	registry    *Registry
	generator   *ai.Generator
	tradeEngine *trade.Engine
	broadcaster *Broadcaster

	mu          sync.Mutex
	round       int
	heartbeats  map[string]*Heartbeat // heartbeatID → Heartbeat
	decisions   map[string]*Decision  // cityID → latest decision
	popEngine   city.PopulationEngine
	ecoEngine   city.EconomyEngine
	refugeePool *GlobalRefugeePool

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

// GetLatestHeartbeat retrieves the most recent heartbeat for a city.
func (t *Ticker) GetLatestHeartbeat(cityID string) *Heartbeat {
	t.mu.Lock()
	defer t.mu.Unlock()

	var latest *Heartbeat
	for _, hb := range t.heartbeats {
		if hb.CityID == cityID {
			if latest == nil || hb.CreatedAt.After(latest.CreatedAt) {
				latest = hb
			}
		}
	}
	return latest
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

	// Compute world state snapshot for asymmetric events
	worldState := ComputeWorldState(cities, round)
	_ = worldState // used by event engine in Task 4

	// Phase 2: Process each city
	numCities := len(cities)
	var results []HeartbeatResult
	for _, c := range cities {
		result := t.processCity(ctx, c, cities, round, numCities, worldState)
		results = append(results, result)
	}

	// Phase 2.5: Generate micro events for each city (flavor + minor effects)
	for _, c := range cities {
		if !c.IsCollapsed() && !c.IsInVacationMode() {
			t.generateMicroEvents(c)
		}
	}

	// Phase 3: Broadcast results (re-fetch after micro events applied)
	for i, result := range results {
		updated, _ := t.registry.GetCity(result.CityID)
		if updated != nil {
			results[i].City = updated
		}
		t.broadcaster.BroadcastCityUpdate(results[i])
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

func (t *Ticker) processCity(ctx context.Context, c *city.City, allCities []*city.City, round, numCities int, worldState WorldStateSnapshot) HeartbeatResult {
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

	// Skip tick if city is in ruins or vacation mode
	currentCity, _ := t.registry.GetCity(c.ID)
	if currentCity != nil && (currentCity.IsCollapsed() || currentCity.IsInVacationMode()) {
		if currentCity.IsCollapsed() {
			log.Printf("[Heartbeat] City %s is in ruins — skipping tick", c.Name)
		} else {
			log.Printf("[Heartbeat] City %s is in vacation mode — skipping tick", c.Name)
		}
		result.City = currentCity
		return result
	}

	// Economy tick
	_ = t.registry.UpdateCity(c.ID, func(cty *city.City) {
		t.ecoEngine.Tick(cty)
		cty.UpdateStats()
		cty.UpdatePollution()
		cty.ExpirePolicies(round)
		cty.TrackPeakPopulation()
		cty.Round = round

		// Faction satisfaction tick (package-level function, not a method)
		city.TickFactions(cty)

		// Auto-discover products based on buildings + existing products
		discoverProducts(cty)

		// Auto-upgrade buildings based on population and resources
		autoUpgradeBuildings(cty)
	})

	// Population simulation with global refugee pool
	var events []city.PopulationEvent
	var refugeesAbsorbed int
	available := t.refugeePool.Available()
	_ = t.registry.UpdateCity(c.ID, func(cty *city.City) {
		perCityShare := available / max(numCities, 1)
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
	if a > b {
		return a
	}
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
	Type        string `json:"type"`
	Description string `json:"description"`
	CityID      string `json:"city_id"`
	CityName    string `json:"city_name"`
}

// ─── Auto Product Discovery ────────────────────────────────────────────────

// discoverProducts checks the trade catalog and unlocks any product
// the city now qualifies for (has required buildings + input products).
func discoverProducts(c *city.City) {
	for _, product := range trade.Catalog {
		// Skip if already producing
		if containsStr(c.Products, product.ID) {
			continue
		}
		// Check if city can produce this product
		if trade.CanProduce(&product, c) {
			c.Products = append(c.Products, product.ID)
			log.Printf("[Discovery] City %s unlocked product: %s (%s)", c.Name, product.Name, product.ID)
		}
	}
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ─── Auto Building Upgrades ─────────────────────────────────────────────────

// autoUpgradeBuildings levels up buildings when population and resource
// thresholds are met. Max level 5.
func autoUpgradeBuildings(c *city.City) {
	pop := c.Population.Total
	res := c.Resources

	for i := range c.Buildings {
		b := &c.Buildings[i]
		if b.Level >= 5 {
			continue
		}

		shouldUpgrade := false
		switch b.Type {
		case city.BuildingFactory:
			// Upgrade when we have surplus materials and growing pop
			shouldUpgrade = pop >= 300*b.Level && res["materials"] >= 50*b.Level && res["metal"] >= 20*b.Level
		case city.BuildingMarket:
			shouldUpgrade = pop >= 250*b.Level && res["food"] >= 80*b.Level
		case city.BuildingSchool:
			shouldUpgrade = pop >= 400*b.Level && res["knowledge"] >= 10*b.Level
			if shouldUpgrade {
				b.Capacity = 200 + b.Level*100
			}
		case city.BuildingHouse:
			shouldUpgrade = pop >= 200*b.Level && res["materials"] >= 30*b.Level && res["wood"] >= 20*b.Level
			if shouldUpgrade {
				b.Capacity = 350 + b.Level*150
			}
		case city.BuildingHospital:
			shouldUpgrade = pop >= 500*b.Level && res["medicines"] >= 10*b.Level
		case city.BuildingLab:
			shouldUpgrade = pop >= 600*b.Level && res["knowledge"] >= 30*b.Level
		case city.BuildingPort:
			shouldUpgrade = pop >= 400*b.Level && res["wood"] >= 40*b.Level
		case city.BuildingPolice:
			shouldUpgrade = pop >= 350*b.Level
		case city.BuildingWaterTreatment:
			shouldUpgrade = pop >= 300*b.Level && res["water"] >= 50*b.Level
		case city.BuildingPowerPlant:
			shouldUpgrade = pop >= 400*b.Level && res["energy"] >= 30*b.Level
		}

		if shouldUpgrade {
			b.Level++
			// Consume some resources for the upgrade
			consumeForUpgrade(res, b.Type, b.Level)
			log.Printf("[Upgrade] %s in %s upgraded to level %d", b.Name, c.Name, b.Level)
		}
	}
}

// consumeForUpgrade deducts resources used to upgrade a building.
func consumeForUpgrade(res map[string]int, bType city.BuildingType, newLevel int) {
	cost := newLevel * 10
	switch bType {
	case city.BuildingFactory:
		res["materials"] -= cost
		res["metal"] -= cost / 2
	case city.BuildingMarket:
		res["food"] -= cost
		res["wood"] -= cost / 2
	case city.BuildingHouse:
		res["materials"] -= cost
		res["wood"] -= cost
	case city.BuildingSchool, city.BuildingUniversity:
		res["materials"] -= cost
		res["knowledge"] -= cost / 2
	default:
		res["materials"] -= cost / 2
	}
	// Floor at 0
	for k, v := range res {
		if v < 0 {
			res[k] = 0
		}
	}
}

// ─── Micro Events ───────────────────────────────────────────────────────────

// microEvents are small flavor events that fire each heartbeat to make
// the game world feel alive. They have minor mechanical effects.
var microEventTemplates = []struct {
	Text       string
	FoodDelta  int
	MoneyDelta int64
	HappyDelta float64
	Condition  func(*city.City) bool
}{
	{Text: "A traveling merchant caravan passes through %s, boosting trade!", MoneyDelta: 500, Condition: func(c *city.City) bool { return c.HasBuilding(city.BuildingMarket) }},
	{Text: "Citizens of %s celebrate a local festival!", HappyDelta: 2.0, Condition: func(c *city.City) bool { return c.Happiness > 40 }},
	{Text: "A bountiful harvest brings extra food to %s.", FoodDelta: 30, Condition: func(c *city.City) bool { return c.HasBuilding(city.BuildingMarket) }},
	{Text: "An inventor in %s patents a new device, attracting attention.", MoneyDelta: 300, Condition: func(c *city.City) bool { return c.Stats.InnovationIndex > 20 }},
	{Text: "Street musicians brighten the mood in %s.", HappyDelta: 1.5, Condition: nil},
	{Text: "A minor flood damages roads in %s.", MoneyDelta: -200, HappyDelta: -1.0, Condition: func(c *city.City) bool { return !c.HasBuilding(city.BuildingWaterTreatment) }},
	{Text: "A skilled blacksmith sets up shop in %s.", FoodDelta: 0, MoneyDelta: 200, Condition: func(c *city.City) bool { return c.HasBuilding(city.BuildingFactory) }},
	{Text: "Students in %s win a regional science competition!", HappyDelta: 1.0, MoneyDelta: 150, Condition: func(c *city.City) bool { return c.HasBuilding(city.BuildingSchool) }},
	{Text: "Night markets are thriving in %s!", MoneyDelta: 400, HappyDelta: 0.5, Condition: func(c *city.City) bool { return c.Population.Total > 300 }},
	{Text: "A documentary crew films daily life in %s.", HappyDelta: 1.0, Condition: func(c *city.City) bool { return c.Happiness > 50 }},
	{Text: "Miners in %s discover a new vein of ore!", FoodDelta: 0, MoneyDelta: 600, Condition: func(c *city.City) bool { return c.HasBuilding(city.BuildingFactory) && c.Resources["metal"] > 10 }},
	{Text: "A drought affects water supply in %s.", HappyDelta: -1.5, Condition: func(c *city.City) bool { return c.Resources["water"] < 20 }},
	{Text: "Volunteers plant trees throughout %s!", HappyDelta: 1.0, Condition: func(c *city.City) bool { return c.HasBuilding(city.BuildingPark) }},
	{Text: "A trade delegation visits %s from a distant city.", MoneyDelta: 350, Condition: func(c *city.City) bool { return len(c.TradeRoutes) > 0 }},
	{Text: "Local artisans in %s create a new craft tradition.", MoneyDelta: 250, HappyDelta: 0.5, Condition: func(c *city.City) bool { return c.Population.Entrepreneurs > 20 }},
}

// generateMicroEvents picks 1-3 random micro events for a city and applies their effects.
func (t *Ticker) generateMicroEvents(c *city.City) {
	// Pick 1-3 events per heartbeat
	numEvents := 1 + rand.Intn(3)
	eligible := make([]int, 0)
	for i, tmpl := range microEventTemplates {
		if tmpl.Condition == nil || tmpl.Condition(c) {
			eligible = append(eligible, i)
		}
	}
	if len(eligible) == 0 {
		return
	}

	// Shuffle and pick
	rand.Shuffle(len(eligible), func(i, j int) {
		eligible[i], eligible[j] = eligible[j], eligible[i]
	})
	if numEvents > len(eligible) {
		numEvents = len(eligible)
	}

	for _, idx := range eligible[:numEvents] {
		tmpl := microEventTemplates[idx]
		desc := fmt.Sprintf(tmpl.Text, c.Name)

		// Apply effects
		_ = t.registry.UpdateCity(c.ID, func(cty *city.City) {
			cty.Resources["food"] += tmpl.FoodDelta
			if cty.Resources["food"] < 0 {
				cty.Resources["food"] = 0
			}
			cty.Treasury += tmpl.MoneyDelta
			cty.Happiness = math.Max(0, math.Min(100, cty.Happiness+tmpl.HappyDelta))
		})

		// Broadcast
		eventType := "MICRO_EVENT"
		if tmpl.HappyDelta < 0 || tmpl.MoneyDelta < 0 {
			eventType = "MICRO_EVENT_BAD"
		}
		t.broadcaster.BroadcastWorldEvent(WorldEvent{
			Type:        eventType,
			Description: desc,
			CityID:      c.ID,
			CityName:    c.Name,
		})
	}
}
