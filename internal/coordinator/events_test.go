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
