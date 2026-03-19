package coordinator

import (
	"math/rand"

	"github.com/cities/game/internal/city"
)

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
