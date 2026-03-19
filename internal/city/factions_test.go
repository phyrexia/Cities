package city

import "testing"

func TestFactionTick_UnemploymentAffectsWorkers(t *testing.T) {
	c := New("TestCity", "mayor1", "Mayor")
	c.Factions = NewFactionSatisfaction()
	c.Stats.UnemploymentRate = 40

	TickFactions(c)

	if c.Factions.Workers >= 55 {
		t.Errorf("expected workers satisfaction < 55, got %d", c.Factions.Workers)
	}
}

func TestFactionTick_DriftToward50(t *testing.T) {
	c := New("TestCity", "mayor1", "Mayor")
	c.Factions = NewFactionSatisfaction()
	c.Factions.Workers = 80
	c.Stats.UnemploymentRate = 15
	// Clear food surplus so it doesn't cancel the drift
	c.Resources["food"] = 0

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
