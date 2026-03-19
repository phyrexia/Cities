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
	for _, v := range []int{f.Workers, f.Business, f.Families} {
		if v > 70 {
			above70++
		}
		if v < 25 {
			below25++
		}
	}
	if f.GreensActive {
		if f.Greens > 70 {
			above70++
		}
		if f.Greens < 25 {
			below25++
		}
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
	if v > 50 {
		return v - 1
	}
	if v < 50 {
		return v + 1
	}
	return v
}

func clampFaction(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
