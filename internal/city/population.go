package city

import (
	"math"
	"math/rand"
)

// ─── Cohort System ───────────────────────────────────────────────────────────
//
// Population is tracked as "cohorts" — groups sharing the same characteristics.
// This avoids per-individual simulation while preserving rich demographic data.

// SocialClass represents a cohort's economic standing.
type SocialClass string

const (
	ClassPoor        SocialClass = "poor"
	ClassWorkingClass SocialClass = "working_class"
	ClassMiddle      SocialClass = "middle"
	ClassUpper       SocialClass = "upper"
)

// CohortType categorizes what kind of people are in the cohort.
type CohortType string

const (
	CohortFounders      CohortType = "founders"      // PERMANENT — original 500, never emigrate
	CohortWorkers       CohortType = "workers"
	CohortFamilies      CohortType = "families"
	CohortEntrepreneurs CohortType = "entrepreneurs"
	CohortStudents      CohortType = "students"
	CohortHomeless      CohortType = "homeless"
	CohortRefugees      CohortType = "refugees"      // arrived from global pool
)

// Population tracks city population via demographic groups.
// Founders (500 originals) are permanently embedded — only their descendants can leave.
type Population struct {
	Total         int `json:"total"`
	Workers       int `json:"workers"`
	Families      int `json:"families"`
	Entrepreneurs int `json:"entrepreneurs"`
	Students      int `json:"students"`
	Homeless      int `json:"homeless"`
	// Founders: the original 500 settlers. These individuals NEVER emigrate.
	// Their children (FounderDescendants) are normal citizens and can emigrate.
	Founders            int `json:"founders"`
	FounderDescendants  int `json:"founder_descendants"`
}

// PopulationEvent describes a migration or demographic event.
type PopulationEvent struct {
	Type   string // ARRIVED, LEFT, BORN, PROMOTED, DIED
	Count  int
	Group  string
	Reason string
}

// RefugeeBatch represents migrants available from the global pool.
type RefugeeBatch struct {
	Count  int
	Reason string // "city_collapsed", "high_crime", "high_tax"
}

// PopulationEngine simulates population changes each heartbeat.
type PopulationEngine struct{}

// Simulate runs one heartbeat of population change.
// refugeesAvailable: migrants from the global pool that this city can absorb.
// Returns events and the number of refugees actually absorbed.
func (pe *PopulationEngine) Simulate(c *City, round int, refugeesAvailable int) ([]PopulationEvent, int) {
	var events []PopulationEvent
	score := c.AttractivenessScore()
	refugeesAbsorbed := 0

	// 1. Natural Growth
	if c.Happiness > 60 && hasBasicNeeds(c) {
		n := naturalGrowth(c)
		if n > 0 {
			// Founders reproduce — children are FounderDescendants (can emigrate)
			founderKids := founderBirths(c, round)
			c.Population.FounderDescendants += founderKids
			// Rest are regular family growth
			regular := n - founderKids
			if regular > 0 {
				c.Population.Families += regular
			}
			c.Population.Total += n
			events = append(events, PopulationEvent{
				Type: "BORN", Count: n,
				Group: "families", Reason: "natural growth",
			})
		}
	}

	// 2. Social Mobility (education enables class upgrade)
	if c.Stats.EducationLevel > 40 {
		mobEvents := pe.simulateSocialMobility(c)
		events = append(events, mobEvents...)
	}

	// 3. Absorb from Global Refugee Pool
	if refugeesAvailable > 0 && score > 40 {
		hasRoom := c.TotalHousingCapacity()-c.Population.Total > 10
		if hasRoom {
			toAbsorb := clamp(int(score*0.04*float64(refugeesAvailable)/100), 1, refugeesAvailable)
			c.Population.Homeless += toAbsorb
			c.Population.Total += toAbsorb
			refugeesAbsorbed = toAbsorb
			events = append(events, PopulationEvent{
				Type: "ARRIVED", Count: toAbsorb,
				Group: "refugees", Reason: "absorbed from global refugee pool",
			})
		}
	}

	// 4. Organic Arrivals
	arrEvents := pe.computeArrivals(c, score)
	events = append(events, arrEvents...)

	// 5. Emigration (founders NEVER emigrate; FounderDescendants can)
	emigEvents, totalLeft := pe.computeEmigration(c)
	events = append(events, emigEvents...)
	_ = totalLeft // coordinator sends these to the global pool

	// 6. Recompute totals
	pe.recomputeTotals(c)

	return events, refugeesAbsorbed
}

func (pe *PopulationEngine) computeArrivals(c *City, score float64) []PopulationEvent {
	var events []PopulationEvent

	if c.HasBuilding(BuildingAffordableHousing) {
		room := c.TotalHousingCapacity() - c.Population.Total
		if room > 20 {
			n := clamp(int(score*0.2), 0, 35)
			if n > 0 {
				c.Population.Families += n/2 + n%2
				c.Population.Homeless += n / 2
				c.Population.Total += n
				events = append(events, PopulationEvent{
					Type: "ARRIVED", Count: n,
					Group: "families/homeless", Reason: "affordable housing available",
				})
			}
		}
	}

	if (c.HasBuilding(BuildingFactory) || c.HasBuilding(BuildingMarket)) && c.Stats.UnemploymentRate < 30 {
		n := clamp(int(score*0.12), 0, 25)
		if n > 0 {
			c.Population.Workers += n
			c.Population.Total += n
			events = append(events, PopulationEvent{
				Type: "ARRIVED", Count: n,
				Group: "workers", Reason: "job opportunities",
			})
		}
	}

	if c.HasBuilding(BuildingUniversity) {
		n := clamp(int(score*0.10), 0, 20)
		if n > 0 {
			c.Population.Students += n * 2 / 3
			c.Population.Entrepreneurs += n / 3
			c.Population.Total += n
			events = append(events, PopulationEvent{
				Type: "ARRIVED", Count: n,
				Group: "students/entrepreneurs", Reason: "university attracts talent",
			})
		}
	}

	if c.TaxRate < 15 {
		n := clamp(int((15-c.TaxRate)*1.2), 0, 12)
		if n > 0 {
			c.Population.Entrepreneurs += n
			c.Population.Total += n
			events = append(events, PopulationEvent{
				Type: "ARRIVED", Count: n,
				Group: "entrepreneurs", Reason: "low tax environment",
			})
		}
	}

	return events
}

func (pe *PopulationEngine) computeEmigration(c *City) ([]PopulationEvent, int) {
	var events []PopulationEvent
	totalLeft := 0

	// Emigratable = everyone except the 500 original founders
	emigratable := c.Population.Total - c.Population.Founders
	if emigratable <= 0 {
		return nil, 0
	}

	doEmigrate := func(n int, group, reason string) {
		n = clamp(n, 0, emigratable-totalLeft)
		if n <= 0 { return }
		emigrateNonFounders(c, n)
		totalLeft += n
		events = append(events, PopulationEvent{
			Type: "LEFT", Count: n, Group: group, Reason: reason,
		})
	}

	if c.TaxRate > 40 {
		doEmigrate(clamp(int((c.TaxRate-40)*2), 0, 50), "mixed", "excessive taxation")
	}
	if c.Happiness < 30 {
		doEmigrate(clamp(int((30-c.Happiness)*1.5), 0, 60), "mixed", "low quality of life")
	}
	if c.Stats.CrimeRate > 65 {
		doEmigrate(clamp(int(float64(c.Stats.CrimeRate-65)*1.2), 0, 45), "families", "high crime rate")
	}
	if c.Treasury < -10000 {
		doEmigrate(clamp(int(math.Abs(float64(c.Treasury))/1000), 0, 80), "workers", "city bankruptcy")
	}

	return events, totalLeft
}

func (pe *PopulationEngine) simulateSocialMobility(c *City) []PopulationEvent {
	var events []PopulationEvent
	rate := float64(c.Stats.EducationLevel) / 1000.0
	promoted := int(float64(c.Population.Workers) * rate * rand.Float64() * 2)
	if promoted > 0 && promoted <= c.Population.Workers {
		c.Population.Workers -= promoted
		c.Population.Entrepreneurs += promoted
		events = append(events, PopulationEvent{
			Type: "PROMOTED", Count: promoted,
			Group: "workers→entrepreneurs",
			Reason: "education-driven social mobility",
		})
	}
	return events
}

func (pe *PopulationEngine) recomputeTotals(c *City) {
	if c.Population.Workers < 0 { c.Population.Workers = 0 }
	if c.Population.Families < 0 { c.Population.Families = 0 }
	if c.Population.Entrepreneurs < 0 { c.Population.Entrepreneurs = 0 }
	if c.Population.Students < 0 { c.Population.Students = 0 }
	if c.Population.Homeless < 0 { c.Population.Homeless = 0 }
	if c.Population.FounderDescendants < 0 { c.Population.FounderDescendants = 0 }

	c.Population.Total = c.Population.Workers + c.Population.Families +
		c.Population.Entrepreneurs + c.Population.Students + c.Population.Homeless
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func naturalGrowth(c *City) int {
	if c.Population.Total < 100 { return 0 }
	rate := 0.005 + (c.Happiness-60)*0.0001
	return int(float64(c.Population.Families) * rate * (0.5 + rand.Float64()))
}

func founderBirths(c *City, round int) int {
	if c.Population.Founders <= 0 || round < 3 { return 0 }
	return int(float64(c.Population.Founders) * 0.003)
}

func hasBasicNeeds(c *City) bool {
	foodItems := []string{"food", "grain", "fish", "meat", "fruit", "bread"}
	for _, p := range c.Products {
		for _, f := range foodItems {
			if p == f { return true }
		}
	}
	return len(c.TradeRoutes) > 0 // can import food
}

func emigrateNonFounders(c *City, count int) {
	emigratable := c.Population.Total - c.Population.Founders
	if emigratable <= 0 || count <= 0 { return }
	ratio := math.Min(float64(count)/float64(emigratable), 0.5)
	c.Population.Workers -= int(float64(c.Population.Workers) * ratio)
	c.Population.Families -= int(float64(c.Population.Families) * ratio)
	c.Population.Entrepreneurs -= int(float64(c.Population.Entrepreneurs) * ratio)
	c.Population.Students -= int(float64(c.Population.Students) * ratio)
	c.Population.Homeless -= int(float64(c.Population.Homeless) * ratio)
	c.Population.FounderDescendants -= int(float64(c.Population.FounderDescendants) * ratio)
}

func clamp(v, min, max int) int {
	if v < min { return min }
	if v > max { return max }
	return v
}
