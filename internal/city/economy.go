package city

import (
	"math"
	"time"
)

// EconomyEngine calculates economic changes each heartbeat.
type EconomyEngine struct{}

// Tick runs one economy cycle: tax collection, wages, expenses.
func (ee *EconomyEngine) Tick(c *City) {
	// Tax income: workers + entrepreneurs pay taxes
	taxBase := int64(c.Population.Workers)*50 + int64(c.Population.Entrepreneurs)*150
	taxIncome := int64(float64(taxBase) * c.TaxRate / 100)
	c.Treasury += taxIncome

	// City services cost: per capita maintenance
	serviceCost := int64(c.Population.Total) * 5
	for _, b := range c.Buildings {
		serviceCost += buildingMaintenanceCost(b)
	}
	c.Treasury -= serviceCost

	// Trade income from active trade routes (simplified)
	tradeIncome := int64(len(c.TradeRoutes)) * int64(len(c.Products)) * 100
	c.Treasury += tradeIncome

	// Entrepreneur bonus: stimulates innovation index
	if c.Population.Entrepreneurs > 50 {
		c.Stats.InnovationIndex = clamp(c.Stats.InnovationIndex+2, 0, 100)
	}

	// GDP calculation
	c.Stats.GDP = taxBase + tradeIncome + int64(c.Stats.InnovationIndex)*500

	// Resource Production
	ee.produceResources(c)

	// Unemployment: jobs from factories vs working-age population
	jobs := jobCapacity(c)
	workingPop := c.Population.Workers + c.Population.Entrepreneurs
	if workingPop > 0 {
		unemployed := workingPop - jobs
		if unemployed < 0 {
			unemployed = 0
		}
		c.Stats.UnemploymentRate = clamp(unemployed*100/workingPop, 0, 100)
	}

}

func (ee *EconomyEngine) produceResources(c *City) {
	if c.Resources == nil {
		c.Resources = make(map[string]int)
	}

	// ─── Building-based production ──────────────────────────────────────
	for _, b := range c.Buildings {
		lvl := b.Level
		if lvl < 1 {
			lvl = 1
		}
		switch b.Type {
		case BuildingFactory:
			c.Resources["materials"] += 10 + (lvl * 5)
			c.Resources["metal"] += 5 + (lvl * 2)
			c.Resources["stone"] += 3 + lvl
		case BuildingMarket:
			c.Resources["food"] += 15 + (lvl * 3)
			c.Resources["wood"] += 5 + lvl // trade brings timber
		case BuildingLab:
			c.Resources["medicines"] += 2 + lvl
			c.Resources["knowledge"] += 3 + (lvl * 2)
		case BuildingHospital:
			c.Resources["medicines"] += 1 + lvl
		case BuildingSchool:
			c.Resources["knowledge"] += 2 + lvl
		case BuildingUniversity:
			c.Resources["knowledge"] += 5 + (lvl * 3)
		case BuildingPort:
			// Ports generate diverse resources via imports
			c.Resources["food"] += 5 + lvl
			c.Resources["materials"] += 3 + lvl
			c.Resources["wood"] += 4 + lvl
		case BuildingWaterTreatment:
			c.Resources["water"] += 20 + (lvl * 8)
		case BuildingPowerPlant:
			c.Resources["energy"] += 15 + (lvl * 5)
		case BuildingPark:
			// Parks produce small amount of food (community gardens)
			c.Resources["food"] += 2
		}
	}

	// ─── Passive production from population activity ────────────────────
	// Citizens naturally gather basic resources
	pop := c.Population.Total
	if pop > 0 {
		c.Resources["wood"] += pop / 200   // foraging
		c.Resources["stone"] += pop / 300  // quarrying
		c.Resources["water"] += pop / 150  // wells
		c.Resources["food"] += pop / 250   // subsistence farming
	}

	// Entrepreneurs generate energy/materials passively
	if c.Population.Entrepreneurs > 10 {
		c.Resources["materials"] += c.Population.Entrepreneurs / 20
		c.Resources["energy"] += c.Population.Entrepreneurs / 30
	}

	// ─── Consumption ────────────────────────────────────────────────────
	foodConsumed := pop / 40 // slightly higher consumption for tension
	c.Resources["food"] -= foodConsumed
	if c.Resources["food"] < 0 {
		c.Resources["food"] = 0
	}

	// Water consumption
	waterConsumed := pop / 80
	c.Resources["water"] -= waterConsumed
	if c.Resources["water"] < 0 {
		c.Resources["water"] = 0
	}

	// Energy consumption (from buildings)
	energyConsumed := len(c.Buildings) * 2
	c.Resources["energy"] -= energyConsumed
	if c.Resources["energy"] < 0 {
		c.Resources["energy"] = 0
		// No penalty, just lack of bonus
	}

	// ─── Resource bonuses ───────────────────────────────────────────────
	// Having energy → productivity bonus (extra materials)
	if c.Resources["energy"] > 10 {
		c.Resources["materials"] += 3
		c.Resources["metal"] += 1
	}
	// Knowledge drives innovation
	if c.Resources["knowledge"] > 20 {
		c.Stats.InnovationIndex = clamp(c.Stats.InnovationIndex+1, 0, 100)
	}
}

// ApplyInitiativeEffects applies the effects of a chosen initiative.
func (c *City) ApplyInitiativeEffects(eff InitiativeEffects, round int) {
	c.Population.Total += eff.PopulationDelta
	if eff.PopulationDelta > 0 {
		c.Population.Workers += eff.PopulationDelta / 2
		c.Population.Families += eff.PopulationDelta / 2
	}
	c.Happiness = math.Max(0, math.Min(100, c.Happiness+eff.HappinessDelta))
	c.Treasury += eff.TreasuryDelta

	for _, prod := range eff.NewProducts {
		if !contains(c.Products, prod) {
			c.Products = append(c.Products, prod)
		}
	}
	for _, bType := range eff.NewBuildings {
		c.AddBuilding(Building{
			Type:     bType,
			Name:     buildingName(bType),
			Level:    1,
			Capacity: buildingDefaultCapacity(bType),
		})
	}

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

	// Track active policy
	c.ActivePolicies = append(c.ActivePolicies, ActivePolicy{
		InitiativeID: eff.Description, // repurposed as label
		Title:        eff.Description,
		Effects:      eff,
		ExpiresAt:    round + 3, // lasts 3 heartbeats by default
	})
}

// ExpirePolicies removes policies that have passed their duration.
func (c *City) ExpirePolicies(currentRound int) {
	active := c.ActivePolicies[:0]
	for _, p := range c.ActivePolicies {
		if p.ExpiresAt > currentRound {
			active = append(active, p)
		}
	}
	c.ActivePolicies = active
}

// UpdateStats recomputes derived city stats from buildings and population.
func (c *City) UpdateStats() {
	if c.HasBuilding(BuildingSchool) {
		c.Stats.EducationLevel = clamp(c.Stats.EducationLevel+3, 0, 100)
	}
	if c.HasBuilding(BuildingUniversity) {
		c.Stats.EducationLevel = clamp(c.Stats.EducationLevel+5, 0, 100)
	}
	if c.HasBuilding(BuildingHospital) {
		c.Stats.HealthLevel = clamp(c.Stats.HealthLevel+4, 0, 100)
	}
	if c.HasBuilding(BuildingPolice) {
		c.Stats.CrimeRate = clamp(c.Stats.CrimeRate-5, 0, 100)
	} else {
		// Crime grows without police
		c.Stats.CrimeRate = clamp(c.Stats.CrimeRate+2, 0, 100)
	}

	// Medicines improve health even without hospital
	if c.Resources != nil && c.Resources["medicines"] > 5 {
		c.Stats.HealthLevel = clamp(c.Stats.HealthLevel+1, 0, 100)
	}
	// Knowledge improves education passively
	if c.Resources != nil && c.Resources["knowledge"] > 10 {
		c.Stats.EducationLevel = clamp(c.Stats.EducationLevel+1, 0, 100)
	}
	// Water treatment improves health
	if c.HasBuilding(BuildingWaterTreatment) {
		c.Stats.HealthLevel = clamp(c.Stats.HealthLevel+2, 0, 100)
	}
}

func buildingMaintenanceCost(b Building) int64 {
	switch b.Type {
	case BuildingHouse:
		return 50
	case BuildingAffordableHousing:
		return 80
	case BuildingSchool:
		return 200
	case BuildingUniversity:
		return 500
	case BuildingHospital:
		return 400
	case BuildingFactory:
		return 150
	case BuildingPolice:
		return 300
	case BuildingLab:
		return 600
	case BuildingPort:
		return 250
	default:
		return 50
	}
}

func jobCapacity(c *City) int {
	jobs := 0
	for _, b := range c.Buildings {
		switch b.Type {
		case BuildingFactory:
			jobs += 100
		case BuildingMarket:
			jobs += 50
		case BuildingSchool:
			jobs += 20
		case BuildingUniversity:
			jobs += 40
		case BuildingHospital:
			jobs += 60
		case BuildingPort:
			jobs += 80
		}
	}
	return jobs
}

func buildingName(t BuildingType) string {
	switch t {
	case BuildingHouse:
		return "Residential Complex"
	case BuildingAffordableHousing:
		return "Affordable Housing Block"
	case BuildingSchool:
		return "Public School"
	case BuildingUniversity:
		return "City University"
	case BuildingHospital:
		return "General Hospital"
	case BuildingFactory:
		return "Industrial Factory"
	case BuildingMarket:
		return "Trade Market"
	case BuildingPark:
		return "City Park"
	case BuildingPolice:
		return "Police Station"
	case BuildingLab:
		return "Research Lab"
	case BuildingPort:
		return "Trade Port"
	default:
		return string(t)
	}
}

func buildingDefaultCapacity(t BuildingType) int {
	switch t {
	case BuildingHouse:
		return 200
	case BuildingAffordableHousing:
		return 300
	case BuildingSchool:
		return 300
	case BuildingUniversity:
		return 500
	case BuildingHospital:
		return 200
	default:
		return 0
	}
}

// InitiativeEffects describes the impact of applying an initiative.
type InitiativeEffects struct {
	PopulationDelta int
	HappinessDelta  float64
	TreasuryDelta   int64
	JobsDelta       int
	NewProducts     []string
	NewBuildings    []BuildingType
	FactionDeltas   map[string]int `json:"faction_deltas,omitempty"`
	Description     string
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ─── Vacation Mode ────────────────────────────────────────────────────────────

// ActivateVacationMode freezes the city for 'days' days.
// While frozen, no tick processing occurs — city state is preserved.
// This is a premium feature (subscription or one-time purchase).
func (c *City) ActivateVacationMode(days int) {
	c.Vacation.Active = true
	c.Vacation.StartedAt = time.Now()
	c.Vacation.ExpiresAt = time.Now().Add(time.Duration(days) * 24 * time.Hour)
	c.Status = StatusVacation
}

// DeactivateVacationMode resumes normal city operation.
func (c *City) DeactivateVacationMode() {
	c.Vacation.Active = false
	c.Status = StatusActive
}

// ─── Pollution Engine ─────────────────────────────────────────────────────────

// UpdatePollution calculates pollution from factories and vehicles.
func (c *City) UpdatePollution() {
	pollution := 0
	for _, b := range c.Buildings {
		switch b.Type {
		case BuildingFactory:
			pollution += 10
		case BuildingPowerPlant:
			pollution += 8
		}
	}
	// Parks and labs reduce pollution
	if c.HasBuilding(BuildingPark) {
		pollution -= 5
	}
	if c.HasBuilding(BuildingLab) {
		pollution -= 3
	}
	// Cleantech product reduces pollution significantly
	for _, p := range c.Products {
		if p == "cleantech" || p == "solar_panels" {
			pollution -= 8
		}
	}
	c.Stats.PollutionLevel = clamp(pollution, 0, 100)
}
