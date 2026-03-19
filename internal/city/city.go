package city

import (
	"time"

	"github.com/google/uuid"
)

// ─── Constants ───────────────────────────────────────────────────────────────

const (
	FounderCount        = 500    // permanent settler count — never emigrate
	BankruptcyThreshold = -50000 // treasury debt that triggers collapse risk
	CollapsedPopulation = 0      // city dies at 0 non-founder population
)

// BuildingType identifies the type of a building.
type BuildingType string

const (
	BuildingHouse             BuildingType = "house"
	BuildingAffordableHousing BuildingType = "affordable_housing"
	BuildingSchool            BuildingType = "school"
	BuildingUniversity        BuildingType = "university"
	BuildingHospital          BuildingType = "hospital"
	BuildingFactory           BuildingType = "factory"
	BuildingMarket            BuildingType = "market"
	BuildingPark              BuildingType = "park"
	BuildingPolice            BuildingType = "police"
	BuildingLab               BuildingType = "lab"
	BuildingPort              BuildingType = "port"
	BuildingMonument          BuildingType = "monument" // unlocked on city recovery
	BuildingWaterTreatment    BuildingType = "water_treatment"
	BuildingPowerPlant        BuildingType = "power_plant"
	BuildingWasteManagement   BuildingType = "waste_management"
)

// CityStatus represents the lifecycle state of a city.
type CityStatus string

const (
	StatusActive     CityStatus = "active"
	StatusVacation   CityStatus = "vacation"   // frozen — needs Vacation Mode subscription
	StatusCollapsing CityStatus = "collapsing" // < 10% population or bankrupt
	StatusRuins      CityStatus = "ruins"      // dead city — can be recovered
)

// Building represents a constructed building in the city.
type Building struct {
	ID       string       `json:"id"`
	Type     BuildingType `json:"type"`
	Name     string       `json:"name"`
	Level    int          `json:"level"`
	Capacity int          `json:"capacity"`
}

// CityStats holds derived metrics computed each heartbeat.
type CityStats struct {
	UnemploymentRate int   `json:"unemployment_rate"` // 0–100
	CrimeRate        int   `json:"crime_rate"`        // 0–100
	EducationLevel   int   `json:"education_level"`   // 0–100
	HealthLevel      int   `json:"health_level"`      // 0–100
	GDP              int64 `json:"gdp"`
	InnovationIndex  int   `json:"innovation_index"` // 0–100
	PollutionLevel   int   `json:"pollution_level"`  // 0–100
}

// ActivePolicy tracks an active policy or initiative effect.
type ActivePolicy struct {
	InitiativeID string
	Title        string
	Effects      InitiativeEffects
	ExpiresAt    int
}

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

// VacationMode holds the freeze state for a city.
type VacationMode struct {
	Active    bool      `json:"active"`
	StartedAt time.Time `json:"started_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RuinsData holds information about a collapsed city.
type RuinsData struct {
	FormerName     string    `json:"former_name"`
	FormerMayorID  string    `json:"former_mayor_id"`
	PeakPopulation int       `json:"peak_population"`
	CollapsedAt    time.Time `json:"collapsed_at"`
	CollapsedRound int       `json:"collapsed_round"`
	RecoveryCost   int64     `json:"recovery_cost"` // Citycoins to reclaim
	HasMonument    bool      `json:"has_monument"`
}

// City is the core domain object representing a player's city.
type City struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	MayorID   string     `json:"mayor_id"`
	MayorName string     `json:"mayor_name"`
	Status    CityStatus `json:"status"`

	Population  Population     `json:"population"`
	Happiness   float64        `json:"happiness"` // 0–100
	Treasury    int64          `json:"treasury"`  // Citycoins
	TaxRate     float64        `json:"tax_rate"`  // 0–100 %
	Buildings   []Building     `json:"buildings"`
	Products    []string       `json:"products"`  // product IDs produced
	Resources   map[string]int `json:"resources"` // ResourceID -> Quantity
	TradeRoutes []string       `json:"trade_routes"`

	Stats          CityStats      `json:"stats"`
	ActivePolicies []ActivePolicy `json:"active_policies"`
	Factions        FactionSatisfaction `json:"factions"`
	ActiveEvents    []GameEvent         `json:"active_events"`
	PendingChains   []PendingChain      `json:"pending_chains"`
	DecisionHistory []DecisionRecord    `json:"decision_history"`
	Mood            string              `json:"mood"`
	Vacation       VacationMode   `json:"vacation"`
	Ruins          *RuinsData     `json:"ruins,omitempty"`

	PeakPopulation int `json:"peak_population"` // all-time max, tracked for leaderboard

	Round     int       `json:"round"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// New creates a new city with starter values including 500 permanent founders.
func New(cityName, mayorID, mayorName string) *City {
	return &City{
		ID:        uuid.NewString(),
		Name:      cityName,
		MayorID:   mayorID,
		MayorName: mayorName,
		Status:    StatusActive,
		Population: Population{
			// 500 founders: counted as workers + families
			Total:         FounderCount,
			Workers:       200,
			Families:      150,
			Entrepreneurs: 30,
			Students:      70,
			Homeless:      50,
			Founders:      FounderCount, // permanent — never emigrate
		},
		Happiness: 60.0,
		Treasury:  10000,
		TaxRate:   20.0,
		Buildings: starterBuildings(),
		Products:  []string{"food", "clothing", "grain", "wood", "stone", "education_svc"},
		Resources: map[string]int{
			"materials": 100,
			"metal":     50,
			"food":      200,
			"wood":      80,
			"stone":     60,
			"water":     150,
			"energy":    0,
			"knowledge": 0,
		},
		TradeRoutes: []string{},
		Stats: CityStats{
			UnemploymentRate: 15,
			CrimeRate:        20,
			EducationLevel:   30,
			HealthLevel:      40,
			PollutionLevel:   10,
		},
		Factions:       NewFactionSatisfaction(),
		PeakPopulation: FounderCount,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// NewFromRuins creates a city by reclaiming ruins.
// Costs recoveryCost Citycoins; grants a Monument building.
func NewFromRuins(ruins *City, newMayorID, newMayorName, newCityName string) *City {
	c := New(newCityName, newMayorID, newMayorName)
	c.ID = ruins.ID // keep same city ID for history continuity
	// Starter boost from ruins: some salvaged materials
	c.Treasury += 2000
	// Grant monument (cosmetic + small happiness bonus)
	c.Buildings = append(c.Buildings, Building{
		ID:    uuid.NewString(),
		Type:  BuildingMonument,
		Name:  "Memorial of " + ruins.Name,
		Level: 1,
	})
	c.Ruins = &RuinsData{
		FormerName:     ruins.Name,
		FormerMayorID:  ruins.MayorID,
		PeakPopulation: ruins.PeakPopulation,
		CollapsedAt:    ruins.UpdatedAt,
		HasMonument:    true,
	}
	return c
}

// Collapse transitions the city to Ruins status.
func (c *City) Collapse(round int) {
	c.Status = StatusRuins
	c.Ruins = &RuinsData{
		FormerName:     c.Name,
		FormerMayorID:  c.MayorID,
		PeakPopulation: c.PeakPopulation,
		CollapsedAt:    time.Now(),
		CollapsedRound: round,
		RecoveryCost:   ruinsRecoveryCost(c),
		HasMonument:    false,
	}
}

// ruinsRecoveryCost calculates how much it costs to recover this city.
func ruinsRecoveryCost(c *City) int64 {
	base := int64(5000)
	// Larger peak population = more expensive to recover (more infrastructure)
	base += int64(c.PeakPopulation / 10)
	// Cap at 50,000 Citycoins
	if base > 50000 {
		base = 50000
	}
	return base
}

// IsCollapsed returns true if the city has died.
func (c *City) IsCollapsed() bool {
	return c.Status == StatusRuins
}

// IsInVacationMode returns true if the city is frozen.
func (c *City) IsInVacationMode() bool {
	return c.Vacation.Active && time.Now().Before(c.Vacation.ExpiresAt)
}

// CheckCollapse evaluates if the city should collapse this tick.
// Founders can never bring the city back to life alone — they survive but city enters ruins.
func (c *City) CheckCollapse(round int) bool {
	if c.Status == StatusRuins {
		return false
	}
	nonFounderPop := c.Population.Total - c.Population.Founders
	if c.Round > 5 && nonFounderPop <= 0 && c.Population.Founders > 0 {
		// Only founders left — city collapses structurally
		c.Collapse(round)
		return true
	}
	if c.Treasury < BankruptcyThreshold {
		c.Status = StatusCollapsing
	}
	return false
}

func starterBuildings() []Building {
	return []Building{
		{ID: uuid.NewString(), Type: BuildingHouse, Name: "Residential Area", Level: 1, Capacity: 350},
		{ID: uuid.NewString(), Type: BuildingMarket, Name: "City Market", Level: 1},
		{ID: uuid.NewString(), Type: BuildingSchool, Name: "Public School", Level: 1, Capacity: 200},
		{ID: uuid.NewString(), Type: BuildingFactory, Name: "Small Workshop", Level: 1},
		{ID: uuid.NewString(), Type: BuildingPolice, Name: "Town Watch", Level: 1},
	}
}

// ─── Query Helpers ────────────────────────────────────────────────────────────

func (c *City) HasBuilding(t BuildingType) bool {
	for _, b := range c.Buildings {
		if b.Type == t {
			return true
		}
	}
	return false
}

func (c *City) AddBuilding(b Building) {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	c.Buildings = append(c.Buildings, b)
}

func (c *City) TotalHousingCapacity() int {
	total := 0
	for _, b := range c.Buildings {
		if b.Type == BuildingHouse || b.Type == BuildingAffordableHousing {
			total += b.Capacity
		}
	}
	return total
}

func (c *City) AttractivenessScore() float64 {
	score := c.Happiness * 0.4
	score += float64(100-c.Stats.UnemploymentRate) * 0.2
	score += float64(c.Stats.EducationLevel) * 0.15
	score += float64(c.Stats.HealthLevel) * 0.1
	score += float64(100-c.Stats.CrimeRate) * 0.1
	score += float64(100-c.Stats.PollutionLevel) * 0.05
	if c.TaxRate > 30 {
		score -= (c.TaxRate - 30) * 0.4
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// TrackPeakPopulation updates the all-time peak.
func (c *City) TrackPeakPopulation() {
	if c.Population.Total > c.PeakPopulation {
		c.PeakPopulation = c.Population.Total
	}
}

// GetProducts returns the list of products the city produces.
// Satisfies the trade.CanProduce interface.
func (c *City) GetProducts() []string {
	return c.Products
}
