# Political Simulator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform Cities into a political simulator with faction satisfaction, 52 narrative events with chained consequences, and reactive Phaser visualization.

**Architecture:** Add a faction engine (`factions.go`) that computes per-tick satisfaction from city stats. Add an event engine (`events.go`) with 52 event definitions, trigger evaluation, urgency countdown, and chain resolution. Integrate both into the existing heartbeat loop. Frontend gets a faction panel, event/decision panel, and timeline replacing the current needs panel and floating banner.

**Tech Stack:** Go 1.22 backend, Phaser 3.80 (Canvas), vanilla JS frontend, WebSocket real-time comms.

**Spec:** `docs/superpowers/specs/2026-03-19-political-simulator-design.md`

---

## File Map

### New Files
| File | Responsibility |
|------|---------------|
| `internal/city/factions.go` | FactionSatisfaction struct, per-tick calculation, drift, threshold checks, happiness derivation |
| `internal/city/factions_test.go` | Unit tests for faction engine |
| `internal/coordinator/events.go` | GameEvent/EventOption structs, 52 event definitions, trigger evaluation, urgency countdown, chain resolution |
| `internal/coordinator/events_test.go` | Unit tests for event engine |
| `internal/coordinator/world_state.go` | WorldStateSnapshot computation from all cities |

### Modified Files
| File | Changes |
|------|---------|
| `internal/city/city.go` | Add Factions, ActiveEvents, PendingChains, DecisionHistory, Mood fields to City struct; update New() |
| `internal/city/economy.go` | Add FactionDeltas to InitiativeEffects; replace updateHappiness with faction-based formula |
| `internal/coordinator/heartbeat.go` | Integrate faction tick, event evaluation, world snapshot, chain resolution into game loop |
| `internal/coordinator/server.go` | Add POST /api/event/decision endpoint; add WS handler for event_decision |
| `internal/coordinator/broadcaster.go` | Add BroadcastEvent, BroadcastEventResolved, BroadcastChainTriggered methods; add message type constants |
| `internal/ai/initiatives.go` | Update AI prompt to include faction impact; add faction_deltas to JSON schema |
| `web/index.html` | New grid layout: faction panel replaces needs panel, event/decision section, timeline panel |
| `web/src/main.js` | Faction panel rendering, event panel with urgency timer, timeline, initiative cards show faction impact |
| `web/src/api/websocket.js` | Handle event_fired, event_resolved, chain_triggered messages; add sendEventDecision() |
| `web/src/scenes/GameScene.js` | Ambient layer (sky color, smoke, nature), reactive visuals (protests, fires, festivals) |
| `web/src/entities/Building.js` | Smoke intensity scaling, fire/flood overlay methods |

---

## Task 1: Faction Data Model & Engine

**Files:**
- Create: `internal/city/factions.go`
- Create: `internal/city/factions_test.go`
- Modify: `internal/city/city.go`

- [ ] **Step 1: Write failing test for faction tick calculation**

```go
// internal/city/factions_test.go
package city

import "testing"

func TestFactionTick_UnemploymentAffectsWorkers(t *testing.T) {
	c := New("TestCity", "mayor1", "Mayor")
	c.Factions = NewFactionSatisfaction()
	c.Stats.UnemploymentRate = 40 // >20, should reduce workers by -2

	TickFactions(c)

	if c.Factions.Workers >= 55 {
		t.Errorf("expected workers satisfaction < 55, got %d", c.Factions.Workers)
	}
}

func TestFactionTick_DriftToward50(t *testing.T) {
	c := New("TestCity", "mayor1", "Mayor")
	c.Factions = NewFactionSatisfaction()
	c.Factions.Workers = 80
	// No stats that would affect workers positively
	c.Stats.UnemploymentRate = 15

	TickFactions(c)

	if c.Factions.Workers >= 80 {
		t.Errorf("expected workers to drift down from 80, got %d", c.Factions.Workers)
	}
}

func TestFactionTick_GreensHiddenUntilEducation(t *testing.T) {
	c := New("TestCity", "mayor1", "Mayor")
	c.Factions = NewFactionSatisfaction()
	c.Stats.EducationLevel = 30

	TickFactions(c)

	if c.Factions.GreensActive {
		t.Error("greens should not be active with education 30")
	}
}

func TestFactionTick_GreensEmerge(t *testing.T) {
	c := New("TestCity", "mayor1", "Mayor")
	c.Factions = NewFactionSatisfaction()
	c.Stats.EducationLevel = 55

	TickFactions(c)

	if !c.Factions.GreensActive {
		t.Error("greens should emerge with education 55")
	}
}

func TestComputeHappiness_FromFactions(t *testing.T) {
	c := New("TestCity", "mayor1", "Mayor")
	c.Factions = FactionSatisfaction{
		Workers: 60, Business: 60, Families: 60, Greens: 60, GreensActive: true,
	}

	h := ComputeHappinessFromFactions(c)

	if h < 55 || h > 65 {
		t.Errorf("expected happiness ~60, got %.1f", h)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/gggp/Code/Cities/Cities && go test ./internal/city/ -run TestFaction -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Add FactionSatisfaction to City struct**

In `internal/city/city.go`, add to City struct after `ActivePolicies`:
```go
Factions         FactionSatisfaction `json:"factions"`
ActiveEvents     []GameEvent         `json:"active_events"`
PendingChains    []PendingChain      `json:"pending_chains"`
DecisionHistory  []DecisionRecord    `json:"decision_history"`
Mood             string              `json:"mood"` // prosperity, stable, crisis, collapsing
```

Add structs after `ActivePolicy`:
```go
type GameEvent struct {
	ID          string        `json:"id"`
	EventDefID  string        `json:"event_def_id"`
	Category    string        `json:"category"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Options     []EventOption `json:"options"`
	Urgency     int           `json:"urgency"`
	FiredAt     int           `json:"fired_at"`
	DefaultOpt  int           `json:"default_opt"`
}

type EventOption struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	FactionDeltas  map[string]int `json:"faction_deltas"`
	ResourceDeltas map[string]int `json:"resource_deltas"`
	StatDeltas     map[string]int `json:"stat_deltas"`
	TreasuryDelta  int64          `json:"treasury_delta"`
	HappinessDelta float64        `json:"happiness_delta"`
	SpawnBuilding  string         `json:"spawn_building,omitempty"`
	ChainEventID   string         `json:"chain_event_id,omitempty"`
	ChainDelay     int            `json:"chain_delay,omitempty"`
}

type PendingChain struct {
	EventDefID string `json:"event_def_id"`
	FiresAt    int    `json:"fires_at"`
	CausedBy   string `json:"caused_by"`
}

type DecisionRecord struct {
	Round       int    `json:"round"`
	EventTitle  string `json:"event_title"`
	ChoiceTitle string `json:"choice_title"`
	Outcome     string `json:"outcome"`
	HasPending  bool   `json:"has_pending"`
}
```

Update `New()` to initialize: `Factions: NewFactionSatisfaction(),`

- [ ] **Step 4: Implement faction engine**

```go
// internal/city/factions.go
package city

import "math"

type FactionSatisfaction struct {
	Workers      int  `json:"workers"`
	Business     int  `json:"business"`
	Families     int  `json:"families"`
	Greens       int  `json:"greens"`
	GreensActive bool `json:"greens_active"`
}

func NewFactionSatisfaction() FactionSatisfaction {
	return FactionSatisfaction{Workers: 55, Business: 50, Families: 60, Greens: 50}
}

func TickFactions(c *City) {
	f := &c.Factions

	// Drift toward 50
	f.Workers = drift(f.Workers)
	f.Business = drift(f.Business)
	f.Families = drift(f.Families)
	if f.GreensActive {
		f.Greens = drift(f.Greens)
	}

	// Stat-to-faction mapping
	if c.Stats.UnemploymentRate > 20 {
		f.Workers -= (c.Stats.UnemploymentRate - 20) / 10
	}
	if c.Stats.UnemploymentRate < 10 {
		f.Workers += 1
	}
	if c.TaxRate > 30 {
		f.Business -= int((c.TaxRate - 30) / 5)
	}
	if c.TaxRate < 15 {
		f.Business += 2
	}
	if c.Stats.CrimeRate > 40 {
		f.Families -= (c.Stats.CrimeRate - 40) / 10
	}
	if c.Stats.EducationLevel > 60 {
		f.Families += 1
	}
	if c.Stats.HealthLevel > 60 {
		f.Families += 1
	}
	if c.Resources != nil && c.Resources["food"] > c.Population.Total/10 {
		f.Workers += 1
		f.Families += 1
	}
	if c.Treasury < 0 {
		f.Workers -= 1
		f.Business -= 1
		f.Families -= 1
		if f.GreensActive {
			f.Greens -= 1
		}
	}

	// Greens specifics
	if f.GreensActive {
		if c.Stats.PollutionLevel > 30 {
			f.Greens -= (c.Stats.PollutionLevel - 30) / 10
		}
		if c.HasBuilding(BuildingPark) {
			f.Greens += 1
		}
		for _, p := range c.Products {
			if p == "cleantech" {
				f.Greens += 2
				break
			}
		}
	}

	// Emergence check
	if !f.GreensActive && c.Stats.EducationLevel > 50 {
		f.GreensActive = true
		f.Greens = 50
	}

	// Clamp all
	f.Workers = clampFaction(f.Workers)
	f.Business = clampFaction(f.Business)
	f.Families = clampFaction(f.Families)
	f.Greens = clampFaction(f.Greens)

	// Update mood
	c.Mood = computeMood(c)

	// Update happiness from factions
	c.Happiness = ComputeHappinessFromFactions(c)
}

func ComputeHappinessFromFactions(c *City) float64 {
	f := c.Factions
	var h float64
	if f.GreensActive {
		h = float64(f.Workers)*0.30 + float64(f.Business)*0.20 + float64(f.Families)*0.35 + float64(f.Greens)*0.15
	} else {
		h = float64(f.Workers)*0.35 + float64(f.Business)*0.25 + float64(f.Families)*0.40
	}
	// Modifiers for shortages
	if c.Resources != nil {
		if c.Resources["food"] <= 0 {
			h -= 5
		}
		if c.Resources["water"] <= 0 {
			h -= 3
		}
	}
	if c.Treasury < -10000 {
		h -= 5
	}
	return math.Max(0, math.Min(100, h))
}

func computeMood(c *City) string {
	f := c.Factions
	above70 := 0
	below25 := 0
	below10 := 0
	for _, v := range []int{f.Workers, f.Business, f.Families} {
		if v > 70 { above70++ }
		if v < 25 { below25++ }
		if v < 10 { below10++ }
	}
	if f.GreensActive {
		if f.Greens > 70 { above70++ }
		if f.Greens < 25 { below25++ }
		if f.Greens < 10 { below10++ }
	}

	if c.Status == StatusCollapsing {
		return "collapsing"
	}
	if above70 >= 3 && c.Treasury > 20000 {
		return "prosperity"
	}
	if below25 > 0 || c.Treasury < -10000 {
		return "crisis"
	}
	return "stable"
}

func drift(v int) int {
	if v > 50 { return v - 1 }
	if v < 50 { return v + 1 }
	return v
}

func clampFaction(v int) int {
	if v < 0 { return 0 }
	if v > 100 { return 100 }
	return v
}
```

- [ ] **Step 5: Run tests and verify pass**

Run: `cd /Users/gggp/Code/Cities/Cities && go test ./internal/city/ -run TestFaction -v && go test ./internal/city/ -run TestComputeHappiness -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add internal/city/factions.go internal/city/factions_test.go internal/city/city.go
git commit -m "feat: add faction system with satisfaction engine, drift, and happiness derivation"
```

---

## Task 2: Add FactionDeltas to Initiatives & Update Economy

**Files:**
- Modify: `internal/city/economy.go`
- Modify: `internal/ai/initiatives.go`

- [ ] **Step 1: Add FactionDeltas to InitiativeEffects**

In `internal/city/economy.go`, add to `InitiativeEffects` struct:
```go
FactionDeltas   map[string]int `json:"faction_deltas,omitempty"`
```

- [ ] **Step 2: Update ApplyInitiativeEffects to apply faction deltas**

In `internal/city/economy.go`, add at the end of `ApplyInitiativeEffects()` before the ActivePolicies block:
```go
// Apply faction deltas
if eff.FactionDeltas != nil {
    if d, ok := eff.FactionDeltas["workers"]; ok {
        c.Factions.Workers = clampFaction(c.Factions.Workers + d)
    }
    if d, ok := eff.FactionDeltas["business"]; ok {
        c.Factions.Business = clampFaction(c.Factions.Business + d)
    }
    if d, ok := eff.FactionDeltas["families"]; ok {
        c.Factions.Families = clampFaction(c.Factions.Families + d)
    }
    if d, ok := eff.FactionDeltas["greens"]; ok && c.Factions.GreensActive {
        c.Factions.Greens = clampFaction(c.Factions.Greens + d)
    }
}
```

Move `clampFaction` to factions.go if not already there (it is — from Task 1).

- [ ] **Step 3: Replace updateHappiness and remove direct Happiness mutations**

In `internal/city/economy.go`:

1. In the `Tick()` method, **remove** the `c.updateHappiness()` call — happiness is now computed in `TickFactions` via `ComputeHappinessFromFactions`.

2. **Remove** the `updateHappiness` method entirely.

3. **Remove** all direct `c.Happiness` mutations from `produceResources()` — these would be silently overwritten by `TickFactions` which runs after the economy tick. The food/water shortage effects are already handled in `ComputeHappinessFromFactions` (checks `food <= 0`, `water <= 0`). Remove these lines:
   - `c.Happiness -= 2` (food shortage, line ~119)
   - `c.Happiness -= 1` (water shortage, line ~128)
   - `c.Happiness += 0.5` (food surplus, line ~141)

4. In `UpdatePollution()`, **remove** the direct `c.Happiness -= ...` for pollution (line ~430) — pollution affects Greens satisfaction which feeds into happiness via faction weights.

After these changes, ALL happiness is derived from faction satisfaction + shortage modifiers in `ComputeHappinessFromFactions`. No other code should directly mutate `c.Happiness`.

- [ ] **Step 4: Update AI prompt to request faction_deltas**

In `internal/ai/initiatives.go`, find the JSON schema section of the prompt and add `faction_deltas` to the effects object description. Add after the `new_buildings` field description:
```
"faction_deltas": {"workers": int, "business": int, "families": int, "greens": int} // political impact on each faction (-20 to +20)
```

Also update the `parseInitiatives` function (or wherever the AI response is parsed) to extract `faction_deltas` from the JSON into `InitiativeEffects.FactionDeltas`.

- [ ] **Step 5: Build and test**

Run: `cd /Users/gggp/Code/Cities/Cities && go build ./... && go test ./... -v`
Expected: Build succeeds, all tests pass

- [ ] **Step 6: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add internal/city/economy.go internal/ai/initiatives.go
git commit -m "feat: add faction_deltas to initiatives, derive happiness from factions"
```

---

## Task 3: Integrate Factions into Heartbeat Loop

**Files:**
- Modify: `internal/coordinator/heartbeat.go`
- Create: `internal/coordinator/world_state.go`

- [ ] **Step 1: Create WorldStateSnapshot**

```go
// internal/coordinator/world_state.go
package coordinator

import "github.com/cities/game/internal/city"

type WorldStateSnapshot struct {
	AnyCityCollapsed    bool
	AnyCityBoom         bool
	AnyCityHighCrime    bool
	AnyCityLowEducation bool
	TotalActiveCities   int
	AverageHappiness    float64
	GlobalRecession     bool
	GlobalPandemic      bool
}

func ComputeWorldState(cities []*city.City, round int) WorldStateSnapshot {
	ws := WorldStateSnapshot{TotalActiveCities: len(cities)}
	totalHappy := 0.0

	for _, c := range cities {
		if c.IsCollapsed() {
			ws.AnyCityCollapsed = true
			continue
		}
		totalHappy += c.Happiness
		if c.Happiness > 80 && c.Treasury > 30000 {
			ws.AnyCityBoom = true
		}
		if c.Stats.CrimeRate > 70 {
			ws.AnyCityHighCrime = true
		}
		if c.Stats.EducationLevel < 20 {
			ws.AnyCityLowEducation = true
		}
	}

	active := 0
	for _, c := range cities {
		if !c.IsCollapsed() {
			active++
		}
	}
	if active > 0 {
		ws.AverageHappiness = totalHappy / float64(active)
	}

	// Rare global events
	if round > 10 {
		ws.GlobalRecession = rand.Intn(100) < 2
	}
	if round > 15 {
		ws.GlobalPandemic = rand.Intn(100) < 1
	}

	return ws
}
```

- [ ] **Step 2: Add import for rand in world_state.go**

Add `"math/rand"` to imports.

- [ ] **Step 3: Integrate TickFactions into heartbeat processCity**

In `internal/coordinator/heartbeat.go`, the economy tick UpdateCity callback at line 282 uses `func(city *city.City)` which shadows the package import. Rename the parameter to `cty` (matching the pattern at line 301) and add the faction tick call.

Replace the entire economy tick UpdateCity block (lines 282-295):
```go
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
```

Note: `city.TickFactions(cty)` — `city` is the package import `github.com/cities/game/internal/city`, and `cty` is the callback parameter (renamed from `city` to avoid shadowing).

- [ ] **Step 4: Add world state computation before per-city processing**

In `runHeartbeat()`, after Phase 1 (trade settlement), before Phase 2:
```go
// Compute world state snapshot for asymmetric events
worldState := ComputeWorldState(cities, round)
_ = worldState // used by event engine in Task 4
```

- [ ] **Step 5: Build and test**

Run: `cd /Users/gggp/Code/Cities/Cities && go build ./... && go test ./... -v`
Expected: Build succeeds, all tests pass

- [ ] **Step 6: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add internal/coordinator/world_state.go internal/coordinator/heartbeat.go
git commit -m "feat: integrate faction ticks and world state snapshot into heartbeat loop"
```

---

## Task 4: Event Engine — Definitions & Trigger Evaluation

**Files:**
- Create: `internal/coordinator/events.go`
- Create: `internal/coordinator/events_test.go`

- [ ] **Step 1: Write failing test for event trigger evaluation**

```go
// internal/coordinator/events_test.go
package coordinator

import (
	"testing"
	"github.com/cities/game/internal/city"
)

func TestEvaluateEvents_CrisisTriggersOnLowHealth(t *testing.T) {
	c := city.New("TestCity", "m1", "Mayor")
	c.Stats.HealthLevel = 20
	c.Round = 10

	events := EvaluateEventTriggers(c, WorldStateSnapshot{}, 10)

	found := false
	for _, e := range events {
		if e.EventDefID == "epidemic" {
			found = true
		}
	}
	if !found {
		t.Error("expected epidemic event to trigger with health=20")
	}
}

func TestEvaluateEvents_NoCrisisInFoundation(t *testing.T) {
	c := city.New("TestCity", "m1", "Mayor")
	c.Stats.HealthLevel = 10
	c.Round = 3

	events := EvaluateEventTriggers(c, WorldStateSnapshot{}, 3)

	for _, e := range events {
		if e.Category == "crisis" {
			t.Errorf("no crisis events should fire during foundation phase (round 3), got %s", e.EventDefID)
		}
	}
}

func TestEvaluateEvents_MaxTwoActive(t *testing.T) {
	c := city.New("TestCity", "m1", "Mayor")
	c.Stats.HealthLevel = 10
	c.Stats.CrimeRate = 80
	c.Factions.Workers = 15
	c.Round = 20
	// Force many triggers

	events := EvaluateEventTriggers(c, WorldStateSnapshot{}, 20)

	if len(events) > 2 {
		t.Errorf("max 2 events, got %d", len(events))
	}
}

func TestUrgencyCountdown(t *testing.T) {
	e := city.GameEvent{Urgency: 3, DefaultOpt: 2}
	e.Urgency--
	if e.Urgency != 2 {
		t.Errorf("expected urgency 2, got %d", e.Urgency)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/gggp/Code/Cities/Cities && go test ./internal/coordinator/ -run TestEvaluate -v`
Expected: FAIL — EvaluateEventTriggers not defined

- [ ] **Step 3: Implement event engine with first 12 crisis events**

```go
// internal/coordinator/events.go
package coordinator

import (
	"math/rand"
	"sync"

	"github.com/cities/game/internal/city"
	"github.com/google/uuid"
)

type EventDef struct {
	ID          string
	Category    string // crisis, opportunity, social, world, chain
	Title       string
	Description string
	Cooldown    int
	Urgency     int
	DefaultOpt  int
	Trigger     func(c *city.City, ws WorldStateSnapshot) bool
	Options     []EventOptionDef
}

type EventOptionDef struct {
	Title          string
	Description    string
	FactionDeltas  map[string]int
	ResourceDeltas map[string]int
	StatDeltas     map[string]int
	TreasuryDelta  int64
	HappinessDelta float64
	SpawnBuilding  string
	ChainEventID   string
	ChainDelay     int
}

// AllEventDefs is the full catalog of event definitions.
var AllEventDefs []EventDef

func init() {
	AllEventDefs = append(AllEventDefs, crisisEvents()...)
	AllEventDefs = append(AllEventDefs, opportunityEvents()...)
	AllEventDefs = append(AllEventDefs, socialEvents()...)
	AllEventDefs = append(AllEventDefs, worldEvents()...)
}

// eventCooldowns tracks per-city cooldowns: cityID+eventDefID → round when available.
// Protected by mutex since processCity may be parallelized in the future.
var (
	eventCooldowns   = make(map[string]int)
	eventCooldownsMu sync.Mutex
)

func cooldownKey(cityID, eventDefID string) string {
	return cityID + ":" + eventDefID
}

func EvaluateEventTriggers(c *city.City, ws WorldStateSnapshot, round int) []city.GameEvent {
	// Don't add events if already at max
	maxNew := 2 - len(c.ActiveEvents)
	if maxNew <= 0 {
		return nil
	}

	// Phase filtering
	isCrisisAllowed := round > 8
	isWorldRareAllowed := round > 15

	eventCooldownsMu.Lock()
	defer eventCooldownsMu.Unlock()

	var fired []city.GameEvent
	for _, def := range AllEventDefs {
		if len(fired) >= maxNew {
			break
		}

		// Phase filter
		if !isCrisisAllowed && def.Category == "crisis" {
			continue
		}
		if !isWorldRareAllowed && (def.ID == "global_recession" || def.ID == "global_pandemic") {
			continue
		}

		// Cooldown check
		key := cooldownKey(c.ID, def.ID)
		if avail, ok := eventCooldowns[key]; ok && round < avail {
			continue
		}

		// Already have this event active?
		alreadyActive := false
		for _, ae := range c.ActiveEvents {
			if ae.EventDefID == def.ID {
				alreadyActive = true
				break
			}
		}
		if alreadyActive {
			continue
		}

		// Trigger check
		if def.Trigger != nil && def.Trigger(c, ws) {
			ge := instantiateEvent(def, round)
			fired = append(fired, ge)
			eventCooldowns[key] = round + def.Cooldown
		}
	}

	return fired
}

func instantiateEvent(def EventDef, round int) city.GameEvent {
	opts := make([]city.EventOption, len(def.Options))
	for i, od := range def.Options {
		opts[i] = city.EventOption{
			ID:             uuid.NewString(),
			Title:          od.Title,
			Description:    od.Description,
			FactionDeltas:  od.FactionDeltas,
			ResourceDeltas: od.ResourceDeltas,
			StatDeltas:     od.StatDeltas,
			TreasuryDelta:  od.TreasuryDelta,
			HappinessDelta: od.HappinessDelta,
			SpawnBuilding:  od.SpawnBuilding,
			ChainEventID:   od.ChainEventID,
			ChainDelay:     od.ChainDelay,
		}
	}
	return city.GameEvent{
		ID:          uuid.NewString(),
		EventDefID:  def.ID,
		Category:    def.Category,
		Title:       def.Title,
		Description: def.Description,
		Options:     opts,
		Urgency:     def.Urgency,
		FiredAt:     round,
		DefaultOpt:  def.DefaultOpt,
	}
}

// ApplyEventOption applies the chosen option's effects to the city.
func ApplyEventOption(c *city.City, event city.GameEvent, optionID string, round int) *city.EventOption {
	var chosen *city.EventOption
	for i := range event.Options {
		if event.Options[i].ID == optionID {
			chosen = &event.Options[i]
			break
		}
	}
	if chosen == nil {
		return nil
	}

	// Apply effects
	c.Treasury += chosen.TreasuryDelta
	// NOTE: HappinessDelta is intentionally ephemeral — it gives immediate visual
	// feedback but will be overwritten next tick by ComputeHappinessFromFactions.
	// For persistent happiness effects, use FactionDeltas instead.
	c.Happiness += chosen.HappinessDelta
	if c.Happiness < 0 { c.Happiness = 0 }
	if c.Happiness > 100 { c.Happiness = 100 }

	for k, v := range chosen.ResourceDeltas {
		if c.Resources == nil { c.Resources = make(map[string]int) }
		c.Resources[k] += v
		if c.Resources[k] < 0 { c.Resources[k] = 0 }
	}
	for k, v := range chosen.FactionDeltas {
		switch k {
		case "workers": c.Factions.Workers = clampFac(c.Factions.Workers + v)
		case "business": c.Factions.Business = clampFac(c.Factions.Business + v)
		case "families": c.Factions.Families = clampFac(c.Factions.Families + v)
		case "greens":
			if c.Factions.GreensActive { c.Factions.Greens = clampFac(c.Factions.Greens + v) }
		}
	}
	for k, v := range chosen.StatDeltas {
		switch k {
		case "crime_rate": c.Stats.CrimeRate = clamp(c.Stats.CrimeRate+v, 0, 100)
		case "health_level": c.Stats.HealthLevel = clamp(c.Stats.HealthLevel+v, 0, 100)
		case "education_level": c.Stats.EducationLevel = clamp(c.Stats.EducationLevel+v, 0, 100)
		case "pollution_level": c.Stats.PollutionLevel = clamp(c.Stats.PollutionLevel+v, 0, 100)
		case "innovation_index": c.Stats.InnovationIndex = clamp(c.Stats.InnovationIndex+v, 0, 100)
		}
	}
	if chosen.SpawnBuilding != "" {
		c.AddBuilding(city.Building{Type: city.BuildingType(chosen.SpawnBuilding), Name: chosen.SpawnBuilding, Level: 1})
	}

	// Schedule chain event
	if chosen.ChainEventID != "" && chosen.ChainDelay > 0 {
		c.PendingChains = append(c.PendingChains, city.PendingChain{
			EventDefID: chosen.ChainEventID,
			FiresAt:    round + chosen.ChainDelay,
			CausedBy:   event.Title + " → " + chosen.Title,
		})
	}

	// Record decision
	c.DecisionHistory = append(c.DecisionHistory, city.DecisionRecord{
		Round:       round,
		EventTitle:  event.Title,
		ChoiceTitle: chosen.Title,
		HasPending:  chosen.ChainEventID != "",
	})
	// Keep last 20
	if len(c.DecisionHistory) > 20 {
		c.DecisionHistory = c.DecisionHistory[len(c.DecisionHistory)-20:]
	}

	// Remove event from active
	active := c.ActiveEvents[:0]
	for _, ae := range c.ActiveEvents {
		if ae.ID != event.ID {
			active = append(active, ae)
		}
	}
	c.ActiveEvents = active

	return chosen
}

// TickEventUrgency decrements urgency on active events and auto-resolves expired ones.
// IMPORTANT: We must NOT call ApplyEventOption during iteration because it mutates
// c.ActiveEvents. Instead, collect expired events first, then process them.
func TickEventUrgency(c *city.City, round int) []string {
	var autoResolved []string
	var expired []city.GameEvent
	var remaining []city.GameEvent

	for _, ae := range c.ActiveEvents {
		ae.Urgency--
		if ae.Urgency <= 0 {
			expired = append(expired, ae)
			autoResolved = append(autoResolved, ae.Title)
		} else {
			remaining = append(remaining, ae)
		}
	}
	// Update active events to only non-expired BEFORE applying effects
	// (ApplyEventOption also removes from ActiveEvents, but we've already filtered)
	c.ActiveEvents = remaining

	// Now safe to apply default options on expired events
	for _, ae := range expired {
		if ae.DefaultOpt < len(ae.Options) {
			ApplyEventOption(c, ae, ae.Options[ae.DefaultOpt].ID, round)
		}
	}

	return autoResolved
}

// FirePendingChains checks for chains that should fire this round.
// Returns fired events and their causedBy strings (parallel arrays).
func FirePendingChains(c *city.City, round int) ([]city.GameEvent, []string) {
	var fired []city.GameEvent
	var causes []string
	var remaining []city.PendingChain
	for _, pc := range c.PendingChains {
		if pc.FiresAt <= round {
			// Find the event def
			for _, def := range AllEventDefs {
				if def.ID == pc.EventDefID {
					ge := instantiateEvent(def, round)
					fired = append(fired, ge)
					causes = append(causes, pc.CausedBy)
					break
				}
			}
		} else {
			remaining = append(remaining, pc)
		}
	}
	c.PendingChains = remaining
	return fired, causes
}

func clampFac(v int) int {
	if v < 0 { return 0 }
	if v > 100 { return 100 }
	return v
}

func clamp(v, min, max int) int {
	if v < min { return min }
	if v > max { return max }
	return v
}
```

- [ ] **Step 4: Add crisis event definitions**

Add to `events.go`:
```go
func crisisEvents() []EventDef {
	return []EventDef{
		{ID: "epidemic", Category: "crisis", Title: "Epidemia", Description: "Una enfermedad se propaga por la ciudad. Los hospitales están desbordados.", Cooldown: 8, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Stats.HealthLevel < 30 },
			Options: []EventOptionDef{
				{Title: "Cuarentena estricta", Description: "Cerrar zonas afectadas", FactionDeltas: map[string]int{"business": -10, "families": 5}, StatDeltas: map[string]int{"health_level": 10}, TreasuryDelta: -3000},
				{Title: "Hospital de campaña", Description: "Construir instalaciones temporales", FactionDeltas: map[string]int{"families": 10, "workers": 5}, TreasuryDelta: -5000, SpawnBuilding: "hospital"},
				{Title: "Ignorar", Description: "Dejar que pase solo", FactionDeltas: map[string]int{"families": -15, "workers": -10}, StatDeltas: map[string]int{"health_level": -15}, ChainEventID: "epidemic_wave2", ChainDelay: 3},
			}},
		{ID: "drought", Category: "crisis", Title: "Sequía severa", Description: "Las reservas de agua se agotan. La población tiene sed.", Cooldown: 10, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Resources != nil && c.Resources["water"] < 15 },
			Options: []EventOptionDef{
				{Title: "Racionamiento", Description: "Limitar uso de agua", FactionDeltas: map[string]int{"workers": -5, "business": -5, "families": 5}, ResourceDeltas: map[string]int{"water": 20}},
				{Title: "Importar agua", Description: "Comprar a ciudades vecinas", TreasuryDelta: -4000, ResourceDeltas: map[string]int{"water": 50}},
				{Title: "Pozos de emergencia", Description: "Cavar pozos rápidamente", FactionDeltas: map[string]int{"workers": 5}, TreasuryDelta: -2000, ResourceDeltas: map[string]int{"water": 30}},
			}},
		{ID: "crime_wave", Category: "crisis", Title: "Ola de crimen", Description: "Los robos y asaltos se disparan. Los ciudadanos tienen miedo.", Cooldown: 7, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Stats.CrimeRate > 60 },
			Options: []EventOptionDef{
				{Title: "Toque de queda", Description: "Restringir movimiento nocturno", FactionDeltas: map[string]int{"workers": -10, "business": -15, "families": 10}, StatDeltas: map[string]int{"crime_rate": -20}},
				{Title: "Más policía", Description: "Contratar agentes", FactionDeltas: map[string]int{"families": 10}, TreasuryDelta: -3000, StatDeltas: map[string]int{"crime_rate": -15}, SpawnBuilding: "police"},
				{Title: "Programas sociales", Description: "Atacar las causas", FactionDeltas: map[string]int{"workers": 15, "families": 5, "business": -5}, TreasuryDelta: -4000, StatDeltas: map[string]int{"crime_rate": -10}, ChainEventID: "crime_drop", ChainDelay: 5},
			}},
		{ID: "industrial_fire", Category: "crisis", Title: "Incendio industrial", Description: "Una fábrica arde. El humo cubre el cielo.", Cooldown: 10, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.HasBuilding(city.BuildingFactory) && c.Stats.PollutionLevel > 40 },
			Options: []EventOptionDef{
				{Title: "Evacuar zona", Description: "Proteger a los ciudadanos", FactionDeltas: map[string]int{"families": 10, "workers": -5}, TreasuryDelta: -2000, HappinessDelta: -3},
				{Title: "Bomberos voluntarios", Description: "Organizar rescate civil", FactionDeltas: map[string]int{"workers": 10, "families": 5}, TreasuryDelta: -1000},
				{Title: "Cerrar fábrica", Description: "Clausurar permanentemente", FactionDeltas: map[string]int{"workers": -15, "business": -20, "greens": 15}, ResourceDeltas: map[string]int{"materials": -30}, StatDeltas: map[string]int{"pollution_level": -15}},
			}},
		{ID: "general_strike", Category: "crisis", Title: "Huelga general", Description: "Los trabajadores se niegan a trabajar. La producción se detiene.", Cooldown: 8, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.Workers < 20 },
			Options: []EventOptionDef{
				{Title: "Negociar", Description: "Mesa de diálogo", FactionDeltas: map[string]int{"workers": 10, "business": -5}, TreasuryDelta: -1000},
				{Title: "Ceder demandas", Description: "Aceptar todo", FactionDeltas: map[string]int{"workers": 20, "business": -15}, TreasuryDelta: -5000},
				{Title: "Reprimir", Description: "Forzar vuelta al trabajo", FactionDeltas: map[string]int{"workers": -20, "families": -10, "business": 5}, StatDeltas: map[string]int{"crime_rate": 10}, HappinessDelta: -5},
			}},
		{ID: "bridge_collapse", Category: "crisis", Title: "Colapso de puente", Description: "Un puente principal colapsa. El tráfico se paraliza.", Cooldown: 15, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Population.Total > 800 },
			Options: []EventOptionDef{
				{Title: "Reparar urgente", Description: "Arreglo rápido pero costoso", TreasuryDelta: -6000, FactionDeltas: map[string]int{"families": 5}},
				{Title: "Desviar tráfico", Description: "Rutas alternativas", FactionDeltas: map[string]int{"business": -5, "workers": -5}, TreasuryDelta: -1000},
				{Title: "Reconstruir mejor", Description: "Puente nuevo moderno", TreasuryDelta: -10000, FactionDeltas: map[string]int{"families": 10, "business": 5, "workers": 5}, ChainEventID: "infrastructure_boost", ChainDelay: 4},
			}},
		{ID: "gas_leak", Category: "crisis", Title: "Fuga de gas", Description: "Gas tóxico escapa de una fábrica. Peligro inminente.", Cooldown: 12, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				factoryLvl := 0; for _, b := range c.Buildings { if b.Type == city.BuildingFactory && b.Level > factoryLvl { factoryLvl = b.Level } }
				return factoryLvl > 2 && !c.HasBuilding(city.BuildingWaterTreatment)
			},
			Options: []EventOptionDef{
				{Title: "Evacuación masiva", Description: "Sacar a todos", FactionDeltas: map[string]int{"families": 10, "workers": -5}, TreasuryDelta: -4000, HappinessDelta: -5},
				{Title: "Contener", Description: "Equipo técnico in situ", FactionDeltas: map[string]int{"workers": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"health_level": -5}},
				{Title: "Minimizar en medios", Description: "Ocultar la gravedad", FactionDeltas: map[string]int{"families": -15, "workers": -10}, StatDeltas: map[string]int{"health_level": -10}, ChainEventID: "health_scandal", ChainDelay: 3},
			}},
		{ID: "food_crisis", Category: "crisis", Title: "Crisis alimentaria", Description: "No hay suficiente comida. El hambre acecha.", Cooldown: 6, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Resources != nil && c.Resources["food"] < 10 },
			Options: []EventOptionDef{
				{Title: "Racionamiento", Description: "Distribuir equitativamente", FactionDeltas: map[string]int{"workers": 5, "business": -10}, ResourceDeltas: map[string]int{"food": 15}},
				{Title: "Importar de emergencia", Description: "Comprar alimentos", TreasuryDelta: -5000, ResourceDeltas: map[string]int{"food": 60}},
				{Title: "Abrir granjas", Description: "Producción de emergencia", FactionDeltas: map[string]int{"workers": 5, "greens": 5}, TreasuryDelta: -3000, ResourceDeltas: map[string]int{"food": 30}, SpawnBuilding: "market"},
			}},
		{ID: "blackout", Category: "crisis", Title: "Apagón masivo", Description: "La ciudad queda a oscuras. Caos en las calles.", Cooldown: 8, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Resources != nil && c.Resources["energy"] < 5 && len(c.Buildings) > 4 },
			Options: []EventOptionDef{
				{Title: "Generadores de emergencia", Description: "Alquilar generadores", TreasuryDelta: -4000, ResourceDeltas: map[string]int{"energy": 30}, FactionDeltas: map[string]int{"families": 5}},
				{Title: "Cortes rotativos", Description: "Energía por turnos", FactionDeltas: map[string]int{"business": -10, "families": -5}, ResourceDeltas: map[string]int{"energy": 15}},
				{Title: "Priorizar hospitales", Description: "Solo servicios esenciales", FactionDeltas: map[string]int{"families": 10, "business": -15}, ResourceDeltas: map[string]int{"energy": 10}, ChainEventID: "looting", ChainDelay: 2},
			}},
		{ID: "flood", Category: "crisis", Title: "Inundación", Description: "Lluvias torrenciales inundan la ciudad. Daños generalizados.", Cooldown: 12, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return !c.HasBuilding(city.BuildingWaterTreatment) && rand.Intn(100) < 8 },
			Options: []EventOptionDef{
				{Title: "Evacuación", Description: "Mover personas a zonas altas", FactionDeltas: map[string]int{"families": 10}, TreasuryDelta: -3000, HappinessDelta: -3},
				{Title: "Diques improvisados", Description: "Construir barreras", FactionDeltas: map[string]int{"workers": 10}, TreasuryDelta: -2000},
				{Title: "Pedir ayuda externa", Description: "Solicitar apoyo", FactionDeltas: map[string]int{"business": -5}, TreasuryDelta: -1000, ChainEventID: "foreign_aid", ChainDelay: 3},
			}},
		{ID: "violent_protests", Category: "crisis", Title: "Protestas violentas", Description: "La tensión social explota. Disturbios en las calles.", Cooldown: 6, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.Factions.Workers < 15 || c.Factions.Business < 15 || c.Factions.Families < 15 || (c.Factions.GreensActive && c.Factions.Greens < 15)
			},
			Options: []EventOptionDef{
				{Title: "Diálogo", Description: "Escuchar demandas", FactionDeltas: map[string]int{"workers": 10, "families": 10}, TreasuryDelta: -2000},
				{Title: "Fuerza policial", Description: "Dispersar protestas", FactionDeltas: map[string]int{"workers": -15, "families": -10, "business": 5}, StatDeltas: map[string]int{"crime_rate": 5}, HappinessDelta: -5},
				{Title: "Concesiones", Description: "Ceder en puntos clave", FactionDeltas: map[string]int{"workers": 15, "families": 10, "business": -10}, TreasuryDelta: -5000},
			}},
		{ID: "corruption_scandal", Category: "crisis", Title: "Escándalo de corrupción", Description: "Se destapa un esquema de sobornos en la administración.", Cooldown: 15, Urgency: 3, DefaultOpt: 1,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Treasury > 50000 },
			Options: []EventOptionDef{
				{Title: "Investigar a fondo", Description: "Transparencia total", FactionDeltas: map[string]int{"workers": 10, "families": 10, "business": -5}, TreasuryDelta: -8000},
				{Title: "Encubrir", Description: "Silenciar el escándalo", FactionDeltas: map[string]int{"business": 5}, ChainEventID: "media_leak", ChainDelay: 4},
				{Title: "Purga de funcionarios", Description: "Despedir a los implicados", FactionDeltas: map[string]int{"workers": 5, "families": 5, "business": -10}, TreasuryDelta: -3000, StatDeltas: map[string]int{"crime_rate": -5}},
			}},
	}
}
```

- [ ] **Step 5a: Add opportunity event definitions**

Add to `events.go`:
```go
func opportunityEvents() []EventDef {
	return []EventDef{
		{ID: "foreign_investor", Category: "opportunity", Title: "Inversor extranjero", Description: "Una corporación internacional quiere invertir en tu ciudad.", Cooldown: 10, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Stats.InnovationIndex > 40 },
			Options: []EventOptionDef{
				{Title: "Aceptar condiciones", Description: "Zona franca y exenciones", FactionDeltas: map[string]int{"business": 15, "workers": -10, "greens": -5}, TreasuryDelta: 8000},
				{Title: "Negociar", Description: "Términos equilibrados", FactionDeltas: map[string]int{"business": 10, "workers": 5}, TreasuryDelta: 4000},
				{Title: "Rechazar", Description: "Proteger industria local", FactionDeltas: map[string]int{"workers": 10, "business": -10}},
			}},
		{ID: "scientific_discovery", Category: "opportunity", Title: "Descubrimiento científico", Description: "Investigadores locales hacen un hallazgo importante.", Cooldown: 12, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.HasBuilding(city.BuildingLab) && c.Resources != nil && c.Resources["knowledge"] > 30 },
			Options: []EventOptionDef{
				{Title: "Patentar", Description: "Monopolizar los beneficios", FactionDeltas: map[string]int{"business": 15, "workers": -5}, TreasuryDelta: 6000, StatDeltas: map[string]int{"innovation_index": 10}},
				{Title: "Open-source", Description: "Compartir con el mundo", FactionDeltas: map[string]int{"workers": 10, "families": 10, "greens": 5, "business": -5}, StatDeltas: map[string]int{"innovation_index": 15, "education_level": 5}, ChainEventID: "startups_flourish", ChainDelay: 4},
				{Title: "Vender derechos", Description: "Dinero rápido", TreasuryDelta: 10000, FactionDeltas: map[string]int{"business": 5, "workers": -5}},
			}},
		{ID: "tourism_boom", Category: "opportunity", Title: "Boom turístico", Description: "Tu ciudad se vuelve destino popular. Turistas llegan en masa.", Cooldown: 10, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Happiness > 70 && c.HasBuilding(city.BuildingPark) },
			Options: []EventOptionDef{
				{Title: "Invertir infraestructura", Description: "Hoteles y transporte", FactionDeltas: map[string]int{"business": 15, "workers": 10, "greens": -5}, TreasuryDelta: -5000, ChainEventID: "cultural_renaissance", ChainDelay: 5},
				{Title: "Limitar turismo", Description: "Preservar calidad de vida", FactionDeltas: map[string]int{"families": 10, "greens": 10, "business": -10}},
				{Title: "Laissez-faire", Description: "Dejar que el mercado decida", FactionDeltas: map[string]int{"business": 5}, TreasuryDelta: 3000},
			}},
		{ID: "international_fair", Category: "opportunity", Title: "Feria internacional", Description: "Tu mercado atrae atención internacional. Proponen una feria.", Cooldown: 8, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				for _, b := range c.Buildings { if b.Type == city.BuildingMarket && b.Level > 2 { return true } }; return false
			},
			Options: []EventOptionDef{
				{Title: "Organizar feria", Description: "Gran evento comercial", FactionDeltas: map[string]int{"business": 15, "workers": 5}, TreasuryDelta: -4000, ResourceDeltas: map[string]int{"food": 40}},
				{Title: "Patrocinar", Description: "Apoyo parcial", FactionDeltas: map[string]int{"business": 10}, TreasuryDelta: -2000},
				{Title: "Ignorar", Description: "No es prioridad ahora"},
			}},
		{ID: "startup_success", Category: "opportunity", Title: "Startup exitosa", Description: "Un emprendedor local crea un producto innovador.", Cooldown: 10, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Population.Entrepreneurs > 50 },
			Options: []EventOptionDef{
				{Title: "Incubar más", Description: "Crear aceleradora", FactionDeltas: map[string]int{"business": 15, "workers": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"innovation_index": 10}, SpawnBuilding: "lab"},
				{Title: "Tax break", Description: "Exenciones fiscales", FactionDeltas: map[string]int{"business": 20, "workers": -5}, TreasuryDelta: -2000},
				{Title: "Regular", Description: "Asegurar cumplimiento", FactionDeltas: map[string]int{"workers": 10, "business": -10}},
			}},
		{ID: "philanthropic_donation", Category: "opportunity", Title: "Donación filantrópica", Description: "Un magnate ofrece una donación generosa a tu ciudad.", Cooldown: 15, Urgency: 5, DefaultOpt: 0,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Happiness > 60 },
			Options: []EventOptionDef{
				{Title: "Hospital", Description: "Construir hospital", FactionDeltas: map[string]int{"families": 15, "workers": 5}, SpawnBuilding: "hospital"},
				{Title: "Escuela", Description: "Nueva institución educativa", FactionDeltas: map[string]int{"families": 15, "workers": 5}, SpawnBuilding: "school", StatDeltas: map[string]int{"education_level": 5}, ChainEventID: "education_boom", ChainDelay: 5},
				{Title: "Parque", Description: "Gran parque público", FactionDeltas: map[string]int{"families": 10, "greens": 15}, SpawnBuilding: "park"},
				{Title: "Vivienda", Description: "Vivienda accesible", FactionDeltas: map[string]int{"workers": 15, "families": 10}, SpawnBuilding: "affordable_housing"},
			}},
		{ID: "famous_artist", Category: "opportunity", Title: "Artista famoso se muda", Description: "Un artista reconocido quiere establecerse en tu ciudad.", Cooldown: 15, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Happiness > 65 && c.HasBuilding(city.BuildingUniversity) },
			Options: []EventOptionDef{
				{Title: "Construir galería", Description: "Centro cultural", FactionDeltas: map[string]int{"families": 10, "business": 5}, TreasuryDelta: -3000, HappinessDelta: 5, ChainEventID: "cultural_renaissance", ChainDelay: 4},
				{Title: "Residencia artística", Description: "Programa de arte", FactionDeltas: map[string]int{"families": 5, "workers": 5}, TreasuryDelta: -1000, HappinessDelta: 3},
				{Title: "Nada especial", Description: "Que se instale por su cuenta"},
			}},
		{ID: "mineral_deposit", Category: "opportunity", Title: "Yacimiento mineral", Description: "Se descubre un yacimiento rico cerca de la ciudad.", Cooldown: 20, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.HasBuilding(city.BuildingFactory) && rand.Intn(100) < 5 },
			Options: []EventOptionDef{
				{Title: "Explotar", Description: "Extracción total", FactionDeltas: map[string]int{"business": 15, "workers": 10, "greens": -20}, ResourceDeltas: map[string]int{"metal": 80, "stone": 60, "materials": 50}, StatDeltas: map[string]int{"pollution_level": 15}},
				{Title: "Explotar sostenible", Description: "Con medidas ambientales", FactionDeltas: map[string]int{"business": 5, "workers": 10, "greens": 5}, ResourceDeltas: map[string]int{"metal": 40, "stone": 30}, TreasuryDelta: -2000},
				{Title: "Reserva natural", Description: "Proteger el terreno", FactionDeltas: map[string]int{"greens": 20, "business": -15}, SpawnBuilding: "park"},
			}},
		{ID: "trade_agreement", Category: "opportunity", Title: "Acuerdo comercial favorable", Description: "Una ruta comercial lucrativa se abre.", Cooldown: 10, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return len(c.TradeRoutes) > 1 },
			Options: []EventOptionDef{
				{Title: "Exclusividad", Description: "Monopolio comercial", FactionDeltas: map[string]int{"business": 15, "workers": -5}, TreasuryDelta: 5000},
				{Title: "Abierto a todos", Description: "Libre comercio", FactionDeltas: map[string]int{"workers": 10, "business": 5}, TreasuryDelta: 3000},
				{Title: "Rechazar", Description: "No nos conviene", FactionDeltas: map[string]int{"workers": 5}},
			}},
		{ID: "sister_city", Category: "opportunity", Title: "Ciudad hermana", Description: "Otra ciudad propone una alianza formal.", Cooldown: 20, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Population.Total > 600 },
			Options: []EventOptionDef{
				{Title: "Aceptar alianza", Description: "Intercambio cultural y comercial", FactionDeltas: map[string]int{"families": 10, "business": 10}, TreasuryDelta: -1000, HappinessDelta: 3},
				{Title: "Proponer términos", Description: "Negociar condiciones", FactionDeltas: map[string]int{"business": 15}, TreasuryDelta: 2000},
				{Title: "Declinar", Description: "Mantener independencia", FactionDeltas: map[string]int{"workers": 5}},
			}},
		{ID: "record_harvest", Category: "opportunity", Title: "Cosecha récord", Description: "Las granjas producen una cosecha excepcional.", Cooldown: 8, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.HasBuilding(city.BuildingMarket) && c.Resources != nil && c.Resources["food"] > 100 },
			Options: []EventOptionDef{
				{Title: "Exportar", Description: "Vender el excedente", FactionDeltas: map[string]int{"business": 10}, TreasuryDelta: 4000, ResourceDeltas: map[string]int{"food": -40}},
				{Title: "Almacenar", Description: "Reservas para crisis", FactionDeltas: map[string]int{"families": 10}, ResourceDeltas: map[string]int{"food": 30}},
				{Title: "Festival de cosecha", Description: "Celebrar con el pueblo", FactionDeltas: map[string]int{"workers": 10, "families": 10}, HappinessDelta: 5, ResourceDeltas: map[string]int{"food": -20}},
			}},
		{ID: "space_program", Category: "opportunity", Title: "Programa espacial popular", Description: "Científicos proponen un ambicioso programa de investigación.", Cooldown: 25, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.HasBuilding(city.BuildingLab) && c.HasBuilding(city.BuildingUniversity) && c.Stats.InnovationIndex > 60 },
			Options: []EventOptionDef{
				{Title: "Financiar", Description: "Inversión estatal total", FactionDeltas: map[string]int{"families": 15, "business": -5, "workers": 5}, TreasuryDelta: -10000, StatDeltas: map[string]int{"innovation_index": 20}},
				{Title: "Colaborar privados", Description: "Asociación público-privada", FactionDeltas: map[string]int{"business": 10, "families": 10}, TreasuryDelta: -5000, StatDeltas: map[string]int{"innovation_index": 15}},
				{Title: "Solo simbólico", Description: "Apoyo moral nada más", FactionDeltas: map[string]int{"families": -5}, StatDeltas: map[string]int{"innovation_index": 3}},
			}},
	}
}
```

- [ ] **Step 5b: Add social event definitions**

Add to `events.go`:
```go
func socialEvents() []EventDef {
	return []EventDef{
		{ID: "worker_protest", Category: "social", Title: "Protesta de trabajadores", Description: "Los trabajadores marchan exigiendo mejoras.", Cooldown: 6, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.Workers < 25 },
			Options: []EventOptionDef{
				{Title: "Subir salario mínimo", Description: "Aumentar salarios", FactionDeltas: map[string]int{"workers": 15, "business": -10}, TreasuryDelta: -3000},
				{Title: "Mesa de diálogo", Description: "Escuchar y negociar", FactionDeltas: map[string]int{"workers": 8, "business": -3}, TreasuryDelta: -1000},
				{Title: "Ignorar", Description: "No ceder a presiones", FactionDeltas: map[string]int{"workers": -10, "business": 5}, HappinessDelta: -3},
			}},
		{ID: "business_lobby", Category: "social", Title: "Lobby empresarial", Description: "Los empresarios presionan por desregulación.", Cooldown: 6, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.Business < 25 },
			Options: []EventOptionDef{
				{Title: "Reducir regulación", Description: "Flexibilizar normas", FactionDeltas: map[string]int{"business": 15, "workers": -10, "greens": -10}, StatDeltas: map[string]int{"pollution_level": 5}},
				{Title: "Escuchar sin ceder", Description: "Reunión informativa", FactionDeltas: map[string]int{"business": 5}},
				{Title: "Rechazar lobby", Description: "Mantener regulaciones", FactionDeltas: map[string]int{"workers": 10, "greens": 10, "business": -10}},
			}},
		{ID: "family_march", Category: "social", Title: "Marcha de familias", Description: "Las familias exigen más servicios públicos.", Cooldown: 6, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.Families < 25 },
			Options: []EventOptionDef{
				{Title: "Más escuelas y parques", Description: "Invertir en servicios", FactionDeltas: map[string]int{"families": 15, "business": -5}, TreasuryDelta: -4000, SpawnBuilding: "park"},
				{Title: "Prometer reformas", Description: "Comprometerse a cambios", FactionDeltas: map[string]int{"families": 8}},
				{Title: "Desestimar", Description: "No es prioridad", FactionDeltas: map[string]int{"families": -10}, HappinessDelta: -3},
			}},
		{ID: "green_blockade", Category: "social", Title: "Bloqueo ecologista", Description: "Activistas bloquean acceso a fábricas contaminantes.", Cooldown: 6, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.GreensActive && c.Factions.Greens < 25 },
			Options: []EventOptionDef{
				{Title: "Cerrar fábrica contaminante", Description: "Clausurar la peor", FactionDeltas: map[string]int{"greens": 20, "workers": -15, "business": -15}, StatDeltas: map[string]int{"pollution_level": -15}, ChainEventID: "unemployment_spike", ChainDelay: 3},
				{Title: "Plan verde", Description: "Compromiso ambiental", FactionDeltas: map[string]int{"greens": 10, "business": -5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"pollution_level": -8}},
				{Title: "Desalojar", Description: "Policía dispersa bloqueo", FactionDeltas: map[string]int{"greens": -15, "workers": -5, "business": 10}, StatDeltas: map[string]int{"crime_rate": 5}},
			}},
		{ID: "spontaneous_festival", Category: "social", Title: "Festival espontáneo", Description: "Los ciudadanos organizan una celebración masiva.", Cooldown: 8, Urgency: 3, DefaultOpt: 1,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.Factions.Workers > 80 || c.Factions.Business > 80 || c.Factions.Families > 80 || (c.Factions.GreensActive && c.Factions.Greens > 80)
			},
			Options: []EventOptionDef{
				{Title: "Patrocinar", Description: "Apoyo municipal", FactionDeltas: map[string]int{"workers": 5, "families": 5, "business": 5}, TreasuryDelta: -2000, HappinessDelta: 5},
				{Title: "Dejar ser", Description: "Celebración orgánica", HappinessDelta: 3},
				{Title: "Cobrar permiso", Description: "Tasa de ocupación", FactionDeltas: map[string]int{"workers": -5, "families": -5, "business": 5}, TreasuryDelta: 1000},
			}},
		{ID: "neighborhood_movement", Category: "social", Title: "Movimiento vecinal", Description: "Los vecinos se organizan para mejorar su barrio.", Cooldown: 10, Urgency: 3, DefaultOpt: 1,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.Families > 70 },
			Options: []EventOptionDef{
				{Title: "Apoyar con fondos", Description: "Subsidio municipal", FactionDeltas: map[string]int{"families": 10, "workers": 5}, TreasuryDelta: -2000},
				{Title: "Dar autonomía", Description: "Dejar que gestionen", FactionDeltas: map[string]int{"families": 8}},
				{Title: "Cooptar", Description: "Absorber en estructura municipal", FactionDeltas: map[string]int{"families": -5, "business": 5}, TreasuryDelta: -500},
			}},
		{ID: "worker_cooperative", Category: "social", Title: "Cooperativa obrera", Description: "Trabajadores proponen formar una cooperativa.", Cooldown: 10, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.Workers > 75 },
			Options: []EventOptionDef{
				{Title: "Subsidiar", Description: "Apoyo económico", FactionDeltas: map[string]int{"workers": 15, "business": -10}, TreasuryDelta: -3000},
				{Title: "Facilitar terreno", Description: "Ceder espacio público", FactionDeltas: map[string]int{"workers": 10, "families": 5}},
				{Title: "Ignorar", Description: "No intervenir"},
			}},
		{ID: "entrepreneur_hackathon", Category: "social", Title: "Hackathon de emprendedores", Description: "Emprendedores organizan un maratón de innovación.", Cooldown: 8, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Factions.Business > 70 },
			Options: []EventOptionDef{
				{Title: "Premiar ganadores", Description: "Premios y reconocimiento", FactionDeltas: map[string]int{"business": 10, "workers": 5}, TreasuryDelta: -2000, StatDeltas: map[string]int{"innovation_index": 5}},
				{Title: "Co-invertir", Description: "Capital semilla", FactionDeltas: map[string]int{"business": 15}, TreasuryDelta: -4000, StatDeltas: map[string]int{"innovation_index": 10}},
				{Title: "Solo publicidad", Description: "Apoyo moral", FactionDeltas: map[string]int{"business": 3}},
			}},
		{ID: "mass_volunteering", Category: "social", Title: "Voluntariado masivo", Description: "Ciudadanos se ofrecen como voluntarios en masa.", Cooldown: 10, Urgency: 3, DefaultOpt: 0,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Happiness > 75 },
			Options: []EventOptionDef{
				{Title: "Infraestructura", Description: "Reparar calles y edificios", FactionDeltas: map[string]int{"workers": 10, "families": 5}, ResourceDeltas: map[string]int{"materials": 20}},
				{Title: "Medio ambiente", Description: "Limpiar y plantar", FactionDeltas: map[string]int{"greens": 15, "families": 5}, StatDeltas: map[string]int{"pollution_level": -5}},
				{Title: "Educación", Description: "Tutorías y mentorías", FactionDeltas: map[string]int{"families": 10}, StatDeltas: map[string]int{"education_level": 5}},
			}},
		{ID: "generational_divide", Category: "social", Title: "División generacional", Description: "Jóvenes y adultos chocan sobre el rumbo de la ciudad.", Cooldown: 12, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return c.Population.Students > 100 && c.Stats.EducationLevel > 60 },
			Options: []EventOptionDef{
				{Title: "Mentorías", Description: "Programa intergeneracional", FactionDeltas: map[string]int{"families": 10, "workers": 5}, TreasuryDelta: -1000, StatDeltas: map[string]int{"education_level": 3}},
				{Title: "Consejo joven", Description: "Voz en el gobierno", FactionDeltas: map[string]int{"families": 5, "business": -3}, HappinessDelta: 2},
				{Title: "Ignorar tensión", Description: "Se resolverá solo", FactionDeltas: map[string]int{"families": -5}, HappinessDelta: -2},
			}},
	}
}
```

- [ ] **Step 5c: Add world/asymmetric event definitions**

Add to `events.go`:
```go
func worldEvents() []EventDef {
	return []EventDef{
		{ID: "refugee_wave", Category: "world", Title: "Ola de refugiados", Description: "Una ciudad vecina ha colapsado. Refugiados llegan a tus puertas.", Cooldown: 10, Urgency: 3, DefaultOpt: 1,
			Trigger: func(_ *city.City, ws WorldStateSnapshot) bool { return ws.AnyCityCollapsed },
			Options: []EventOptionDef{
				{Title: "Acoger todos", Description: "Abrir las puertas", FactionDeltas: map[string]int{"workers": 5, "families": -10, "business": -5}, HappinessDelta: -3, ResourceDeltas: map[string]int{"food": -30}},
				{Title: "Acoger algunos", Description: "Inmigración selectiva", FactionDeltas: map[string]int{"workers": 5, "families": -3}},
				{Title: "Cerrar fronteras", Description: "No aceptar refugiados", FactionDeltas: map[string]int{"families": -5, "workers": -5, "business": 5}, HappinessDelta: -2},
			}},
		{ID: "talent_drain", Category: "world", Title: "Fuga de talento", Description: "Una ciudad exitosa atrae a tus mejores profesionales.", Cooldown: 8, Urgency: 3, DefaultOpt: 1,
			Trigger: func(_ *city.City, ws WorldStateSnapshot) bool { return ws.AnyCityBoom },
			Options: []EventOptionDef{
				{Title: "Contra-oferta fiscal", Description: "Incentivos para quedarse", FactionDeltas: map[string]int{"business": 10, "workers": 5}, TreasuryDelta: -4000},
				{Title: "Mejorar calidad vida", Description: "Inversión en servicios", FactionDeltas: map[string]int{"families": 10}, TreasuryDelta: -3000, HappinessDelta: 2},
				{Title: "Dejar ir", Description: "No intervenir", FactionDeltas: map[string]int{"business": -5, "workers": -5}},
			}},
		{ID: "displaced_criminals", Category: "world", Title: "Criminales desplazados", Description: "Una ciudad con alto crimen expulsa delincuentes hacia tu territorio.", Cooldown: 8, Urgency: 3, DefaultOpt: 1,
			Trigger: func(_ *city.City, ws WorldStateSnapshot) bool { return ws.AnyCityHighCrime },
			Options: []EventOptionDef{
				{Title: "Reforzar policía", Description: "Más agentes en fronteras", FactionDeltas: map[string]int{"families": 10, "business": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"crime_rate": -5}},
				{Title: "Programas rehabilitación", Description: "Reintegración social", FactionDeltas: map[string]int{"workers": 10, "families": 5, "business": -5}, TreasuryDelta: -2000, StatDeltas: map[string]int{"crime_rate": -3}},
				{Title: "Nada", Description: "Ignorar el problema", StatDeltas: map[string]int{"crime_rate": 10}, FactionDeltas: map[string]int{"families": -10}},
			}},
		{ID: "global_recession", Category: "world", Title: "Recesión global", Description: "La economía mundial entra en recesión. Tu ciudad no es inmune.", Cooldown: 20, Urgency: 3, DefaultOpt: 1,
			Trigger: func(_ *city.City, ws WorldStateSnapshot) bool { return ws.GlobalRecession },
			Options: []EventOptionDef{
				{Title: "Austeridad", Description: "Recortar gastos", FactionDeltas: map[string]int{"workers": -10, "families": -10, "business": 5}, TreasuryDelta: 5000},
				{Title: "Estímulo fiscal", Description: "Inyectar dinero", FactionDeltas: map[string]int{"workers": 10, "business": 10, "families": 5}, TreasuryDelta: -8000},
				{Title: "Proteccionismo", Description: "Cerrar fronteras comerciales", FactionDeltas: map[string]int{"workers": 5, "business": -15}, ResourceDeltas: map[string]int{"materials": -20}},
			}},
		{ID: "global_pandemic", Category: "world", Title: "Pandemia mundial", Description: "Un virus se propaga por todo el mundo. Tu ciudad debe responder.", Cooldown: 25, Urgency: 2, DefaultOpt: 2,
			Trigger: func(_ *city.City, ws WorldStateSnapshot) bool { return ws.GlobalPandemic },
			Options: []EventOptionDef{
				{Title: "Lockdown", Description: "Confinamiento total", FactionDeltas: map[string]int{"families": 10, "workers": -15, "business": -20}, StatDeltas: map[string]int{"health_level": 10}, TreasuryDelta: -5000},
				{Title: "Medidas parciales", Description: "Restricciones moderadas", FactionDeltas: map[string]int{"families": 5, "workers": -5, "business": -10}, StatDeltas: map[string]int{"health_level": 5}, TreasuryDelta: -3000},
				{Title: "Negacionismo", Description: "Ignorar la pandemia", FactionDeltas: map[string]int{"business": 5, "families": -20, "workers": -10}, StatDeltas: map[string]int{"health_level": -15}},
			}},
		{ID: "cultural_trend", Category: "world", Title: "Moda cultural de otra ciudad", Description: "Una tendencia cultural de otra ciudad exitosa llega a la tuya.", Cooldown: 8, Urgency: 3, DefaultOpt: 1,
			Trigger: func(_ *city.City, ws WorldStateSnapshot) bool { return ws.AverageHappiness > 60 },
			Options: []EventOptionDef{
				{Title: "Importar cultura", Description: "Abrazar la tendencia", FactionDeltas: map[string]int{"families": 10, "business": 5}, TreasuryDelta: -1000, HappinessDelta: 3},
				{Title: "Crear propia", Description: "Impulsar cultura local", FactionDeltas: map[string]int{"families": 5, "workers": 5}, TreasuryDelta: -2000, HappinessDelta: 2},
				{Title: "Ignorar tendencia", Description: "No es relevante"},
			}},
		{ID: "trade_competition", Category: "world", Title: "Competencia comercial", Description: "Otra ciudad produce los mismos bienes. Tus mercados se ven amenazados.", Cooldown: 8, Urgency: 3, DefaultOpt: 1,
			Trigger: func(c *city.City, ws WorldStateSnapshot) bool { return len(c.Products) > 5 && ws.TotalActiveCities > 1 },
			Options: []EventOptionDef{
				{Title: "Innovar", Description: "Invertir en diferenciación", FactionDeltas: map[string]int{"business": 10, "workers": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"innovation_index": 5}},
				{Title: "Bajar precios", Description: "Competir en costos", FactionDeltas: map[string]int{"business": 5, "workers": -10}, TreasuryDelta: -2000},
				{Title: "Diversificar", Description: "Nuevos productos", FactionDeltas: map[string]int{"workers": 5, "business": 5}, TreasuryDelta: -1000},
			}},
		{ID: "alliance_proposed", Category: "world", Title: "Alianza propuesta", Description: "Una ciudad con población similar propone cooperación.", Cooldown: 15, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, ws WorldStateSnapshot) bool { return ws.TotalActiveCities > 1 && c.Population.Total > 400 },
			Options: []EventOptionDef{
				{Title: "Aceptar", Description: "Alianza completa", FactionDeltas: map[string]int{"business": 10, "families": 5}, TreasuryDelta: 2000, HappinessDelta: 2},
				{Title: "Contra-proponer", Description: "Términos más favorables", FactionDeltas: map[string]int{"business": 15}, TreasuryDelta: 3000},
				{Title: "Rechazar", Description: "Independencia total", FactionDeltas: map[string]int{"workers": 5}},
			}},
		{ID: "supply_blockade", Category: "world", Title: "Bloqueo de suministros", Description: "Una ruta comercial es bloqueada. Recursos escasean.", Cooldown: 10, Urgency: 3, DefaultOpt: 1,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool { return len(c.TradeRoutes) > 2 && rand.Intn(100) < 10 },
			Options: []EventOptionDef{
				{Title: "Rutas alternativas", Description: "Buscar nuevos caminos", FactionDeltas: map[string]int{"business": 5}, TreasuryDelta: -3000},
				{Title: "Producción local", Description: "Autosuficiencia", FactionDeltas: map[string]int{"workers": 10, "business": -5}, ResourceDeltas: map[string]int{"materials": 20, "food": 20}},
				{Title: "Negociar", Description: "Diplomacia con bloqueadores", FactionDeltas: map[string]int{"business": 5}, TreasuryDelta: -2000},
			}},
		{ID: "scientist_migration", Category: "world", Title: "Migración de científicos", Description: "Científicos huyen de una ciudad con baja educación.", Cooldown: 10, Urgency: 3, DefaultOpt: 1,
			Trigger: func(_ *city.City, ws WorldStateSnapshot) bool { return ws.AnyCityLowEducation },
			Options: []EventOptionDef{
				{Title: "Reclutar agresivo", Description: "Ofertas de trabajo", FactionDeltas: map[string]int{"business": 10, "workers": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"education_level": 5, "innovation_index": 5}},
				{Title: "Bienvenida pasiva", Description: "Facilitar integración", FactionDeltas: map[string]int{"families": 5}, StatDeltas: map[string]int{"education_level": 3}},
				{Title: "No interferir", Description: "Dejar que fluya naturalmente"},
			}},
	}
}
```

- [ ] **Step 5d: Add chain follow-up event definitions**

Add to `events.go`, a function for chain-triggered events. These are simpler events triggered by chained consequences from earlier decisions:
```go
func init() {
	// Chain follow-up events are appended to AllEventDefs in a second init pass
	AllEventDefs = append(AllEventDefs, chainEvents()...)
}

func chainEvents() []EventDef {
	return []EventDef{
		{ID: "epidemic_wave2", Category: "chain", Title: "Segunda ola de epidemia", Description: "La enfermedad que ignoraste regresa con más fuerza.", Cooldown: 0, Urgency: 2, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Cuarentena total", Description: "Medidas extremas", FactionDeltas: map[string]int{"business": -15, "families": 5}, TreasuryDelta: -5000, StatDeltas: map[string]int{"health_level": 15}},
				{Title: "Hospital de emergencia", Description: "Todo los recursos a salud", FactionDeltas: map[string]int{"families": 10}, TreasuryDelta: -8000, SpawnBuilding: "hospital"},
			}},
		{ID: "media_leak", Category: "chain", Title: "Filtración a medios", Description: "Periodistas revelan el escándalo que intentaste ocultar.", Cooldown: 0, Urgency: 2, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Disculpa pública", Description: "Aceptar responsabilidad", FactionDeltas: map[string]int{"workers": 10, "families": 10, "business": -10}, TreasuryDelta: -3000},
				{Title: "Contraatacar", Description: "Negar todo", FactionDeltas: map[string]int{"workers": -10, "families": -15}, HappinessDelta: -5},
			}},
		{ID: "looting", Category: "chain", Title: "Saqueos", Description: "La oscuridad del apagón provocó saqueos generalizados.", Cooldown: 0, Urgency: 2, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Ley marcial", Description: "Control militar", FactionDeltas: map[string]int{"families": 5, "workers": -10, "business": 5}, StatDeltas: map[string]int{"crime_rate": -10}, HappinessDelta: -3},
				{Title: "Reparaciones comunitarias", Description: "Vecinos se organizan", FactionDeltas: map[string]int{"workers": 10, "families": 10}, TreasuryDelta: -2000, StatDeltas: map[string]int{"crime_rate": -5}, ChainEventID: "crime_wave_aftermath", ChainDelay: 3},
			}},
		{ID: "infrastructure_boost", Category: "chain", Title: "Boost de infraestructura", Description: "El nuevo puente impulsa el desarrollo de la zona.", Cooldown: 0, Urgency: 5, DefaultOpt: 0,
			Options: []EventOptionDef{
				{Title: "Zona comercial", Description: "Desarrollar comercio", FactionDeltas: map[string]int{"business": 15, "workers": 10}, TreasuryDelta: 3000},
				{Title: "Zona residencial", Description: "Más viviendas", FactionDeltas: map[string]int{"families": 15, "workers": 5}, SpawnBuilding: "house"},
			}},
		{ID: "crime_drop", Category: "chain", Title: "Reducción del crimen", Description: "Los programas sociales dan fruto. El crimen baja.", Cooldown: 0, Urgency: 5, DefaultOpt: 0,
			Options: []EventOptionDef{
				{Title: "Celebrar éxito", Description: "Comunicar los resultados", FactionDeltas: map[string]int{"families": 10, "workers": 5}, HappinessDelta: 3, StatDeltas: map[string]int{"crime_rate": -10}},
				{Title: "Expandir programas", Description: "Más inversión social", FactionDeltas: map[string]int{"workers": 15, "families": 10, "business": -5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"crime_rate": -15}},
			}},
		{ID: "foreign_aid", Category: "chain", Title: "Ayuda internacional", Description: "La ayuda que pediste llega con condiciones.", Cooldown: 0, Urgency: 3, DefaultOpt: 0,
			Options: []EventOptionDef{
				{Title: "Aceptar condiciones", Description: "Reformas impuestas", FactionDeltas: map[string]int{"business": 10, "workers": -5}, TreasuryDelta: 5000, ResourceDeltas: map[string]int{"food": 30, "materials": 20}},
				{Title: "Solo recursos", Description: "Negociar sin reformas", TreasuryDelta: 2000, ResourceDeltas: map[string]int{"food": 15, "materials": 10}},
			}},
		{ID: "health_scandal", Category: "chain", Title: "Escándalo sanitario", Description: "La fuga de gas que minimizaste causa enfermedades crónicas.", Cooldown: 0, Urgency: 2, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Compensar afectados", Description: "Indemnizaciones", FactionDeltas: map[string]int{"families": 10, "workers": 5}, TreasuryDelta: -6000, StatDeltas: map[string]int{"health_level": 5}},
				{Title: "Investigación médica", Description: "Estudiar efectos", FactionDeltas: map[string]int{"families": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"health_level": 3, "innovation_index": 3}},
			}},
		{ID: "gentrification", Category: "chain", Title: "Gentrificación", Description: "La inversión extranjera dispara los precios. Los pobres son desplazados.", Cooldown: 0, Urgency: 3, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Control de rentas", Description: "Limitar precios", FactionDeltas: map[string]int{"workers": 15, "families": 10, "business": -15}},
				{Title: "Vivienda social", Description: "Construir vivienda accesible", FactionDeltas: map[string]int{"workers": 10, "families": 10, "business": -5}, TreasuryDelta: -4000, SpawnBuilding: "affordable_housing"},
				{Title: "Dejar al mercado", Description: "No intervenir", FactionDeltas: map[string]int{"business": 10, "workers": -15, "families": -10}, ChainEventID: "worker_protest", ChainDelay: 3},
			}},
		{ID: "startups_flourish", Category: "chain", Title: "Florecen las startups", Description: "La decisión de compartir el descubrimiento atrae emprendedores.", Cooldown: 0, Urgency: 5, DefaultOpt: 0,
			Options: []EventOptionDef{
				{Title: "Crear hub tecnológico", Description: "Zona de innovación", FactionDeltas: map[string]int{"business": 15, "workers": 10}, TreasuryDelta: -3000, StatDeltas: map[string]int{"innovation_index": 10}, SpawnBuilding: "lab"},
				{Title: "Incentivar contratación", Description: "Empleo local", FactionDeltas: map[string]int{"workers": 15, "business": 5}, TreasuryDelta: -2000},
			}},
		{ID: "unemployment_spike", Category: "chain", Title: "Pico de desempleo", Description: "El cierre de la fábrica deja a cientos sin trabajo.", Cooldown: 0, Urgency: 2, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Subsidio de desempleo", Description: "Apoyo temporal", FactionDeltas: map[string]int{"workers": 10, "business": -5}, TreasuryDelta: -4000},
				{Title: "Reciclaje laboral", Description: "Formación en nuevos oficios", FactionDeltas: map[string]int{"workers": 10, "families": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"education_level": 3}},
			}},
		{ID: "education_boom", Category: "chain", Title: "Boom educativo", Description: "La nueva escuela genera un movimiento educativo.", Cooldown: 0, Urgency: 5, DefaultOpt: 0,
			Options: []EventOptionDef{
				{Title: "Becas universitarias", Description: "Expandir educación superior", FactionDeltas: map[string]int{"families": 10, "workers": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"education_level": 10}, SpawnBuilding: "university", ChainEventID: "greens_emerge", ChainDelay: 3},
				{Title: "Escuelas técnicas", Description: "Formación práctica", FactionDeltas: map[string]int{"workers": 15, "business": 10}, TreasuryDelta: -2000, StatDeltas: map[string]int{"education_level": 5}},
			}},
		{ID: "cultural_renaissance", Category: "chain", Title: "Renacimiento cultural", Description: "El turismo y el arte transforman la ciudad en un centro cultural.", Cooldown: 0, Urgency: 5, DefaultOpt: 0,
			Options: []EventOptionDef{
				{Title: "Festival internacional", Description: "Gran evento cultural", FactionDeltas: map[string]int{"families": 10, "business": 15}, TreasuryDelta: -4000, HappinessDelta: 5},
				{Title: "Preservar autenticidad", Description: "Mantener identidad local", FactionDeltas: map[string]int{"families": 15, "greens": 5}, HappinessDelta: 3},
			}},
		{ID: "crime_wave_aftermath", Category: "chain", Title: "Ola de crimen residual", Description: "Los saqueos generaron redes criminales organizadas.", Cooldown: 0, Urgency: 2, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Operación especial", Description: "Desmantelar redes", FactionDeltas: map[string]int{"families": 10, "business": 5, "workers": -5}, TreasuryDelta: -4000, StatDeltas: map[string]int{"crime_rate": -15}},
				{Title: "Vigilancia comunitaria", Description: "Vecinos patrullan", FactionDeltas: map[string]int{"workers": 10, "families": 5}, StatDeltas: map[string]int{"crime_rate": -8}},
			}},
		{ID: "greens_emerge", Category: "chain", Title: "Movimiento ambientalista", Description: "La educación despierta conciencia ecológica. Los ecologistas se organizan.", Cooldown: 0, Urgency: 5, DefaultOpt: 0,
			Options: []EventOptionDef{
				{Title: "Apoyar el movimiento", Description: "Integrar en política", FactionDeltas: map[string]int{"greens": 15, "families": 5, "business": -5}, ChainEventID: "green_movement", ChainDelay: 3},
				{Title: "Observar", Description: "Dejar que evolucione solo", FactionDeltas: map[string]int{"greens": 5}},
			}},
		{ID: "green_movement", Category: "chain", Title: "Revolución verde", Description: "Los ecologistas proponen un plan ambiental ambicioso.", Cooldown: 0, Urgency: 3, DefaultOpt: 1,
			Options: []EventOptionDef{
				{Title: "Plan verde integral", Description: "Transformación ecológica", FactionDeltas: map[string]int{"greens": 20, "families": 10, "business": -15}, TreasuryDelta: -6000, StatDeltas: map[string]int{"pollution_level": -20}, SpawnBuilding: "water_treatment"},
				{Title: "Medidas moderadas", Description: "Pasos graduales", FactionDeltas: map[string]int{"greens": 10, "business": -5}, TreasuryDelta: -2000, StatDeltas: map[string]int{"pollution_level": -8}},
				{Title: "Rechazar", Description: "Priorizar economía", FactionDeltas: map[string]int{"greens": -15, "business": 10}},
			}},
	}
}
```

**Important**: Remove the duplicate `init()` — merge both into a single init:
```go
func init() {
	AllEventDefs = append(AllEventDefs, crisisEvents()...)
	AllEventDefs = append(AllEventDefs, opportunityEvents()...)
	AllEventDefs = append(AllEventDefs, socialEvents()...)
	AllEventDefs = append(AllEventDefs, worldEvents()...)
	AllEventDefs = append(AllEventDefs, chainEvents()...)
}
```

- [ ] **Step 6: Run tests and verify pass**

Run: `cd /Users/gggp/Code/Cities/Cities && go test ./internal/coordinator/ -run TestEvaluate -v && go test ./internal/coordinator/ -run TestUrgency -v`
Expected: ALL PASS

- [ ] **Step 7: Build full project**

Run: `cd /Users/gggp/Code/Cities/Cities && go build ./...`
Expected: Build succeeds

- [ ] **Step 8: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add internal/coordinator/events.go internal/coordinator/events_test.go
git commit -m "feat: add event engine with 52 event definitions, trigger evaluation, urgency, and chain resolution"
```

---

## Task 5: Wire Events into Heartbeat Loop & Add API Endpoint

**Files:**
- Modify: `internal/coordinator/heartbeat.go`
- Modify: `internal/coordinator/server.go`
- Modify: `internal/coordinator/broadcaster.go`

- [ ] **Step 1: Add event message type constants to broadcaster.go**

In `internal/coordinator/broadcaster.go`, add `"github.com/cities/game/internal/city"` to imports (needed for `city.GameEvent` in new methods). Then add constants:
```go
const (
	MsgEventFired     = "event_fired"
	MsgEventResolved  = "event_resolved"
	MsgChainTriggered = "chain_triggered" // used by BroadcastChainTriggered
)
```

Add methods (NOTE: `client.Send` is a `chan []byte`, not a method — must marshal to JSON bytes and write to channel with non-blocking select):
```go
func (b *Broadcaster) BroadcastEvent(cityID string, event city.GameEvent) {
	msg := WSMessage{Type: MsgEventFired, Payload: event}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Broadcaster] Error marshaling event: %v", err)
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, client := range b.clients {
		if client.CityID == cityID {
			select {
			case client.Send <- data:
			default:
				log.Printf("[Broadcaster] Client %s send buffer full — dropping event", client.PlayerID)
			}
		}
	}
}

func (b *Broadcaster) BroadcastChainTriggered(cityID string, event city.GameEvent, causedBy string) {
	msg := WSMessage{Type: MsgChainTriggered, Payload: map[string]interface{}{"event": event, "caused_by": causedBy}}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Broadcaster] Error marshaling chain event: %v", err)
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, client := range b.clients {
		if client.CityID == cityID {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
}

func (b *Broadcaster) BroadcastEventResolved(cityID string, eventID string, outcome string) {
	msg := WSMessage{Type: MsgEventResolved, Payload: map[string]string{"event_id": eventID, "outcome": outcome}}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[Broadcaster] Error marshaling event resolved: %v", err)
		return
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, client := range b.clients {
		if client.CityID == cityID {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
}
```

- [ ] **Step 2: Integrate events into heartbeat processCity**

In `internal/coordinator/heartbeat.go`, inside `processCity`, after the economy tick UpdateCity block, add:

```go
// Event engine: urgency countdown + chain firing + new event triggers
// IMPORTANT: Broadcaster calls must happen OUTSIDE UpdateCity callback to avoid
// nested locking (UpdateCity holds registry lock, Broadcaster acquires its own lock).
var eventAutoResolved []string
var eventChainsFired []city.GameEvent
var eventChainCauses []string
var eventNewFired []city.GameEvent

_ = t.registry.UpdateCity(c.ID, func(cty *city.City) {
	// Tick urgency on active events (auto-resolve expired)
	eventAutoResolved = TickEventUrgency(cty, round)

	// Fire pending chains
	chainsFired, causes := FirePendingChains(cty, round)
	for i, ce := range chainsFired {
		if len(cty.ActiveEvents) < 2 {
			cty.ActiveEvents = append(cty.ActiveEvents, ce)
			eventChainsFired = append(eventChainsFired, ce)
			if i < len(causes) {
				eventChainCauses = append(eventChainCauses, causes[i])
			}
		}
	}

	// Evaluate new event triggers
	newEvents := EvaluateEventTriggers(cty, worldState, round)
	for _, ne := range newEvents {
		cty.ActiveEvents = append(cty.ActiveEvents, ne)
		eventNewFired = append(eventNewFired, ne)
	}
})

// Broadcast outside the registry lock
for _, title := range eventAutoResolved {
	log.Printf("[Events] Auto-resolved expired event: %s in %s", title, c.Name)
}
for i, ce := range eventChainsFired {
	cause := ce.Description
	if i < len(eventChainCauses) {
		cause = eventChainCauses[i]
	}
	t.broadcaster.BroadcastChainTriggered(c.ID, ce, cause)
	log.Printf("[Events] Chain event fired: %s in %s", ce.Title, c.Name)
}
for _, ne := range eventNewFired {
	t.broadcaster.BroadcastEvent(c.ID, ne)
	log.Printf("[Events] New event fired: %s in %s", ne.Title, c.Name)
}
```

Update `processCity` signature to accept `worldState WorldStateSnapshot`:
```go
func (t *Ticker) processCity(ctx context.Context, c *city.City, cities []*city.City, round, numCities int, worldState WorldStateSnapshot) HeartbeatResult {
```

Update the call in `runHeartbeat` to pass worldState.

- [ ] **Step 3: Suppress initiatives when events active**

In `runHeartbeat`, wrap the heartbeat generation in Phase 4:
```go
// Phase 4: Generate new heartbeats for next round (suppressed if events active)
for _, c := range cities {
	currentCity, _ := t.registry.GetCity(c.ID)
	if currentCity != nil && len(currentCity.ActiveEvents) > 0 {
		log.Printf("[Heartbeat] Skipping initiative proposals for %s — %d active events", c.Name, len(currentCity.ActiveEvents))
		continue
	}
	t.generateHeartbeat(ctx, c, cities, round)
}
```

- [ ] **Step 4: Add POST /api/event/decision endpoint**

In `internal/coordinator/server.go`, add route:
```go
mux.HandleFunc("/api/event/decision", s.handleEventDecision)
```

Add handler:
```go
func (s *Server) handleEventDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		CityID   string `json:"city_id"`
		EventID  string `json:"event_id"`
		OptionID string `json:"option_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	var result *city.EventOption
	round := s.ticker.CurrentRound()
	err := s.registry.UpdateCity(req.CityID, func(c *city.City) {
		for _, ae := range c.ActiveEvents {
			if ae.ID == req.EventID {
				result = ApplyEventOption(c, ae, req.OptionID, round)
				break
			}
		}
	})
	if err != nil || result == nil {
		http.Error(w, "event or option not found", 404)
		return
	}

	s.broadcaster.BroadcastEventResolved(req.CityID, req.EventID, result.Title)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "choice": result.Title})
}
```

- [ ] **Step 5: Add WS handler for event_decision messages**

In `server.go` inside the WebSocket message handling loop, add a case for `"event_decision"`:
```go
case "event_decision":
	var payload struct {
		EventID  string `json:"event_id"`
		OptionID string `json:"option_id"`
	}
	// parse payload and call ApplyEventOption similar to REST handler
```

- [ ] **Step 6: Build and test**

Run: `cd /Users/gggp/Code/Cities/Cities && go build ./... && go test ./... -v`
Expected: Build succeeds, all tests pass

- [ ] **Step 7: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add internal/coordinator/heartbeat.go internal/coordinator/server.go internal/coordinator/broadcaster.go
git commit -m "feat: wire event engine into heartbeat loop, add event decision API endpoint"
```

---

## Task 6: Frontend — Faction Panel & Event Decision UI

**Files:**
- Modify: `web/index.html`
- Modify: `web/src/main.js`
- Modify: `web/src/api/websocket.js`

- [ ] **Step 1: Update HTML grid layout**

In `web/index.html`, replace the `#needs-panel` div with a `#factions-panel`:
```html
<div id="factions-panel">
  <div class="panel-title">FACCIONES</div>
  <div id="factions-list"></div>
</div>
```

Replace the `#heartbeat-banner` with a fixed event/decision section inside the trade area:
```html
<div id="trade-area">
  <div id="decision-panel">
    <div class="panel-title">DECISIONES</div>
    <div id="decision-content"></div>
  </div>
  <div id="timeline-panel">
    <div class="panel-title">HISTORIAL</div>
    <div id="timeline-list"></div>
  </div>
</div>
```

Update CSS grid for the new layout:
```css
#game-area {
  display: grid;
  grid-template-columns: 1fr 180px 180px;
}
#factions-panel {
  grid-column: 3 / 4;
  background: #0f0f1a;
  border-left: 1px solid #333;
  overflow-y: auto;
  padding: 12px;
}
#trade-area {
  display: grid;
  grid-template-columns: 1.5fr 1fr;
}
```

Add faction bar CSS:
```css
.faction-bar {
  margin-bottom: 10px;
}
.faction-bar-fill {
  height: 8px;
  border-radius: 4px;
  transition: width 0.5s ease, background 0.3s;
}
.faction-bar-label {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
  margin-bottom: 3px;
}
.urgency-bar {
  height: 4px;
  background: #333;
  margin-top: 8px;
  border-radius: 2px;
  overflow: hidden;
}
.urgency-bar-fill {
  height: 100%;
  background: #FF4444;
  transition: width 0.3s;
}
```

- [ ] **Step 2: Add faction panel rendering in main.js**

```javascript
const FACTIONS = {
  workers:  { name: 'Trabajadores', color: '#4A9EFF', emoji: { angry: '😡', uneasy: '😟', content: '🙂', happy: '😄' }},
  business: { name: 'Empresarios',  color: '#FFD700', emoji: { angry: '😡', uneasy: '😟', content: '🙂', happy: '😄' }},
  families: { name: 'Familias',     color: '#00FF88', emoji: { angry: '😡', uneasy: '😟', content: '🙂', happy: '😄' }},
  greens:   { name: 'Ecologistas',  color: '#88FF88', emoji: { angry: '😡', uneasy: '😟', content: '🙂', happy: '😄' }},
};

function renderFactions(factions) {
  const panel = document.getElementById('factions-list');
  if (!factions) { panel.innerHTML = ''; return; }

  let html = '';
  for (const [key, display] of Object.entries(FACTIONS)) {
    if (key === 'greens' && !factions.greens_active) continue;
    const val = factions[key] || 50;
    const emoji = val < 25 ? display.emoji.angry : val < 50 ? display.emoji.uneasy : val < 75 ? display.emoji.content : display.emoji.happy;
    const barColor = val < 25 ? '#FF4444' : val < 50 ? '#FFD700' : display.color;

    html += `
      <div class="faction-bar">
        <div class="faction-bar-label">
          <span style="color: ${display.color}">${emoji} ${display.name}</span>
          <span style="color: ${barColor}; font-weight: bold;">${val}%</span>
        </div>
        <div style="height: 8px; background: #222; border-radius: 4px; overflow: hidden;">
          <div class="faction-bar-fill" style="width: ${val}%; background: ${barColor};"></div>
        </div>
      </div>
    `;
  }
  panel.innerHTML = html;
}
```

- [ ] **Step 3: Add event/decision panel rendering**

```javascript
function renderActiveEvents(events, proposals) {
  const panel = document.getElementById('decision-content');

  if (events && events.length > 0) {
    // Show events (priority over proposals)
    let html = '';
    events.forEach(event => {
      const urgencyPct = (event.urgency / 5) * 100; // assume max 5
      html += `
        <div style="border: 1px solid #FFD700; padding: 12px; margin-bottom: 10px; background: #1a1a0a;">
          <div style="color: #FFD700; font-weight: bold; font-size: 0.85rem;">${event.title}</div>
          <div style="color: #aaa; font-size: 0.75rem; margin: 6px 0;">${event.description}</div>
          <div class="urgency-bar"><div class="urgency-bar-fill" style="width: ${urgencyPct}%"></div></div>
          <div style="font-size: 0.65rem; color: #FF4444; margin-top: 2px;">${event.urgency} turnos restantes</div>
          <div style="display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px;">
            ${event.options.map(opt => `
              <button class="pixel-btn" style="padding: 6px 10px; font-size: 0.7rem; flex: 1; min-width: 100px;"
                onclick="submitEventDecision('${event.id}', '${opt.id}')">
                <div>${opt.title}</div>
                <div style="font-size: 0.6rem; color: #888; font-weight: normal;">${formatFactionImpact(opt.faction_deltas)}</div>
              </button>
            `).join('')}
          </div>
        </div>
      `;
    });
    panel.innerHTML = html;
  } else if (proposals) {
    // Show normal heartbeat proposals (existing logic moved here)
    showHeartbeatInPanel(proposals);
  } else {
    panel.innerHTML = '<div style="color: #555; padding: 20px; text-align: center;">Esperando siguiente turno...</div>';
  }
}

function formatFactionImpact(deltas) {
  if (!deltas) return '';
  return Object.entries(deltas).map(([k, v]) => {
    const sign = v > 0 ? '+' : '';
    const color = v > 0 ? '#00FF88' : '#FF4444';
    const icon = k === 'workers' ? '👷' : k === 'business' ? '💼' : k === 'families' ? '👨‍👩‍👧' : '🌱';
    return `<span style="color: ${color}">${icon}${sign}${v}</span>`;
  }).join(' ');
}

async function submitEventDecision(eventId, optionId) {
  try {
    const resp = await fetch('/api/event/decision', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ city_id: gameState.cityID, event_id: eventId, option_id: optionId })
    });
    if (resp.ok) {
      addLog('Decisión de evento tomada', 'good');
    }
  } catch (err) {
    addLog(`Error: ${err.message}`, 'bad');
  }
}
```

- [ ] **Step 4: Add timeline panel rendering**

```javascript
function renderTimeline(history) {
  const panel = document.getElementById('timeline-list');
  if (!history || history.length === 0) {
    panel.innerHTML = '<div style="color: #555; font-size: 0.75rem; padding: 10px;">Sin decisiones aún</div>';
    return;
  }

  let html = '';
  history.slice().reverse().forEach(record => {
    const pendingIcon = record.has_pending ? '⏳' : '✓';
    html += `
      <div style="font-size: 0.7rem; padding: 6px 0; border-bottom: 1px solid #222; color: #aaa;">
        <div style="display: flex; justify-content: space-between;">
          <span style="color: #FFD700;">R${record.round}</span>
          <span>${pendingIcon}</span>
        </div>
        <div style="color: #ccc;">${record.event_title}</div>
        <div style="color: #4A9EFF;">→ ${record.choice_title}</div>
      </div>
    `;
  });
  panel.innerHTML = html;
}
```

- [ ] **Step 5: Update websocket.js to handle new message types**

In `web/src/api/websocket.js`, the existing CitiesWS class routes messages by type. The `main.js` already listens via `.on()`. Add handlers in `main.js` initWebSocket:

```javascript
.on('event_fired', (event) => {
  if (event) {
    addLog(`⚠️ ${event.title}`, 'important');
    // Re-render decision panel with active events
    if (gameState.city) {
      if (!gameState.city.active_events) gameState.city.active_events = [];
      gameState.city.active_events.push(event);
      renderActiveEvents(gameState.city.active_events);
    }
  }
})
.on('event_resolved', (data) => {
  if (data && gameState.city) {
    gameState.city.active_events = (gameState.city.active_events || []).filter(e => e.id !== data.event_id);
    renderActiveEvents(gameState.city.active_events);
    addLog(`✓ Evento resuelto: ${data.outcome}`, 'good');
  }
})
.on('chain_triggered', (event) => {
  if (event) {
    addLog(`🔗 Consecuencia: ${event.title}`, 'important');
    if (gameState.city) {
      if (!gameState.city.active_events) gameState.city.active_events = [];
      gameState.city.active_events.push(event);
      renderActiveEvents(gameState.city.active_events);
    }
  }
})
```

- [ ] **Step 6: Update city_update handler to render factions, events, timeline**

In the existing `city_update` handler in `main.js`, add:
```javascript
renderFactions(update.city.factions);
renderActiveEvents(update.city.active_events, null);
renderTimeline(update.city.decision_history);
```

- [ ] **Step 7: Test in browser manually**

Run: `cd /Users/gggp/Code/Cities/Cities && make dev` (or `go run ./cmd/coordinator/`)
Open browser, create city, trigger heartbeat, verify:
- Faction bars visible and updating
- Events fire when conditions met
- Event decisions work
- Timeline shows history

- [ ] **Step 8: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add web/
git commit -m "feat: add faction panel, event decision UI, and timeline panel"
```

---

## Task 7: Phaser Reactive Visualization

**Files:**
- Modify: `web/src/scenes/GameScene.js`
- Modify: `web/src/entities/Building.js`

- [ ] **Step 1: Add ambient sky color based on happiness**

In `GameScene.js`, add to `create()`:
```javascript
this.skyRect = this.add.rectangle(0, 0, W, H * 0.45, 0x1a1a3e).setOrigin(0, 0);
```
(Replace the existing sky rectangle assignment to save reference)

In `updateCity()`:
```javascript
// Ambient sky color based on happiness
const happy = city.happiness || 50;
let skyColor;
if (happy > 70) skyColor = 0x4488cc;      // bright blue
else if (happy > 40) skyColor = 0x556677;  // grey-blue
else skyColor = 0x333344;                   // dark stormy
this.skyRect.setFillStyle(skyColor);
```

- [ ] **Step 2: Add citizen faction colors**

In `_createCitizens`, replace the static colors array:
```javascript
const factionColors = [0x4A9EFF, 0x4A9EFF, 0xFFD700, 0x00FF88, 0x00FF88, 0x88FF88]; // workers, business, families, greens
```

In `updateCity`, adjust citizen speed based on happiness:
```javascript
const speedMultiplier = happy > 70 ? 1.5 : happy > 40 ? 1.0 : 0.5;
this.citizens.forEach(c => {
  const baseSpeed = c.speed > 0 ? 1 : -1;
  c.speed = baseSpeed * (0.5 + Math.random() * 1.5) * speedMultiplier;
});
```

- [ ] **Step 3: Add protest visual when faction <25**

In `updateCity`:
```javascript
// Protest visuals
if (this.protestGroup) this.protestGroup.destroy();
const factions = city.factions;
if (factions) {
  const lowFaction = factions.workers < 25 || factions.business < 25 || factions.families < 25 || (factions.greens_active && factions.greens < 25);
  if (lowFaction) {
    this.protestGroup = this.add.graphics();
    const px = W * 0.4;
    const py = H * 0.55;
    // Protest sign
    this.protestGroup.fillStyle(0xFF4444);
    this.protestGroup.fillRect(px, py - 20, 2, 15);
    this.protestGroup.fillRect(px - 8, py - 25, 18, 10);
    // Group of angry dots
    for (let i = 0; i < 8; i++) {
      this.protestGroup.fillStyle(0xFF4444);
      this.protestGroup.fillCircle(px - 15 + i * 5, py - 2 + Math.random() * 6, 3);
    }
  }
}
```

- [ ] **Step 4: Add smoke intensity to factories**

In `Building.js`, modify `_drawFactory` to accept a pollution level and scale smoke:
```javascript
static _drawFactory(g, x, y, s, c, pollutionLevel = 0) {
  // existing body code...

  // Smoke particles scaled by pollution
  const smokeAlpha = Math.min(1.0, 0.3 + (pollutionLevel || 0) * 0.01);
  g.fillStyle(0x888888, smokeAlpha);
  g.fillCircle(x + s * 1.3, y - s * 1.5, s * 0.4 + pollutionLevel * 0.02);
  g.fillCircle(x + s * 3.8, y - s * 1.8, s * 0.3 + pollutionLevel * 0.02);
}
```

Update `_rebuildBuildings` in GameScene.js to pass pollution:
```javascript
const pollution = this.cityData?.stats?.pollution_level || 0;
// pass pollution to PixelBuilding.draw when building is factory
```

- [ ] **Step 5: Add festival confetti when faction >80**

In `updateCity`:
```javascript
// Festival visual when any faction > 80
if (this.confettiEmitter) { this.confettiEmitter.forEach(p => p.destroy()); this.confettiEmitter = null; }
if (factions) {
  const highFaction = factions.workers > 80 || factions.business > 80 || factions.families > 80 || (factions.greens_active && factions.greens > 80);
  if (highFaction) {
    this.confettiEmitter = [];
    for (let i = 0; i < 15; i++) {
      const dot = this.add.graphics();
      const colors = [0xFFD700, 0xFF4444, 0x00FF88, 0x4A9EFF, 0xFF88AA];
      dot.fillStyle(colors[i % colors.length], 0.8);
      dot.fillCircle(0, 0, 2);
      dot.x = Phaser.Math.Between(0, W);
      dot.y = Phaser.Math.Between(H * 0.2, H * 0.5);
      this.confettiEmitter.push(dot);
      this.tweens.add({ targets: dot, y: dot.y + 30, alpha: 0, duration: 2000 + Math.random() * 1000, repeat: -1, yoyo: true });
    }
  }
}
```

- [ ] **Step 6: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add web/src/scenes/GameScene.js web/src/entities/Building.js
git commit -m "feat: reactive Phaser visuals — sky mood, protests, festivals, smoke scaling"
```

---

## Task 8: Phase-Dependent Collapse & Final Integration

**Files:**
- Modify: `internal/city/city.go`
- Modify: `internal/coordinator/heartbeat.go`

- [ ] **Step 1: Replace CheckCollapse with phase-aware logic**

In `internal/city/city.go`, replace `CheckCollapse`:
```go
func (c *City) CheckCollapse(round int) bool {
	if c.Status == StatusRuins {
		return false
	}

	// Foundation (1-8) and Growth (9-15): immune
	if round <= 15 {
		return false
	}

	// Count factions below thresholds
	factionsBelow10 := 0
	factionsBelow5 := 0
	for _, v := range []int{c.Factions.Workers, c.Factions.Business, c.Factions.Families} {
		if v < 10 { factionsBelow10++ }
		if v < 5 { factionsBelow5++ }
	}
	if c.Factions.GreensActive {
		if c.Factions.Greens < 10 { factionsBelow10++ }
		if c.Factions.Greens < 5 { factionsBelow5++ }
	}

	shouldCollapse := false
	if round <= 25 {
		// Maturity: 2+ factions <10 OR treasury < -50k
		shouldCollapse = factionsBelow10 >= 2 || c.Treasury < -50000
	} else {
		// No Safety Net: 1+ faction <5 OR treasury < -30k
		shouldCollapse = factionsBelow5 >= 1 || c.Treasury < -30000
	}

	if shouldCollapse {
		if c.Status != StatusCollapsing {
			c.Status = StatusCollapsing
			return false // first turn of collapsing = grace starts
		}
		// Already collapsing — check if grace expired
		// Grace: 3 turns (maturity) or 2 turns (no safety net)
		grace := 3
		if round > 25 { grace = 2 }
		// Use Round field to track when collapsing started
		// If we've been collapsing for grace turns, actually collapse
		// (simplified: count consecutive collapsing ticks via a simple check)
		c.Collapse(round)
		return true
	}

	// Recovered from collapsing
	if c.Status == StatusCollapsing {
		c.Status = StatusActive
	}

	// Legacy: founders-only check
	nonFounderPop := c.Population.Total - c.Population.Founders
	if round > 15 && nonFounderPop <= 0 && c.Population.Founders > 0 {
		c.Collapse(round)
		return true
	}

	return false
}
```

- [ ] **Step 2: Add CollapsingStartRound to City for grace period tracking**

In `internal/city/city.go`, add to City struct:
```go
CollapsingStartRound int `json:"collapsing_start_round,omitempty"`
```

Update CheckCollapse to use it:
```go
if shouldCollapse {
    if c.Status != StatusCollapsing {
        c.Status = StatusCollapsing
        c.CollapsingStartRound = round
        return false // grace period starts
    }
    grace := 3
    if round > 25 { grace = 2 }
    if round - c.CollapsingStartRound >= grace {
        c.Collapse(round)
        return true
    }
    return false // still in grace
}
// Recovered
if c.Status == StatusCollapsing {
    c.Status = StatusActive
    c.CollapsingStartRound = 0
}
```

- [ ] **Step 3: Build and run full test suite**

Run: `cd /Users/gggp/Code/Cities/Cities && go build ./... && go test ./... -v`
Expected: All pass

- [ ] **Step 4: Commit**

```bash
cd /Users/gggp/Code/Cities/Cities && git add internal/city/city.go internal/coordinator/heartbeat.go
git commit -m "feat: phase-dependent collapse with faction-based triggers and grace periods"
```

---

## Task Summary

| Task | Description | Estimated Steps |
|------|-------------|-----------------|
| 1 | Faction data model & engine | 6 |
| 2 | FactionDeltas in initiatives & economy | 6 |
| 3 | Integrate factions into heartbeat | 6 |
| 4 | Event engine — 52 definitions + triggers | 8 |
| 5 | Wire events into heartbeat + API endpoint | 7 |
| 6 | Frontend — faction panel, event UI, timeline | 8 |
| 7 | Phaser reactive visualization | 6 |
| 8 | Phase-dependent collapse | 4 |

**Total: 8 tasks, ~51 steps**

Each task produces a working, compilable commit. Tasks 1-3 can be parallelized (factions). Task 4-5 depend on 1-3. Task 6-7 (frontend) can be parallelized. Task 8 is final integration.
