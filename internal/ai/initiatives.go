package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cities/game/internal/city"
	"github.com/cities/game/internal/trade"
)

// InitiativeType categorizes a policy proposal.
type InitiativeType string

const (
	TypeTaxChange  InitiativeType = "TAX_CHANGE"
	TypeHousing    InitiativeType = "HOUSING"
	TypeIndustry   InitiativeType = "INDUSTRY"
	TypeEducation  InitiativeType = "EDUCATION"
	TypeHealth     InitiativeType = "HEALTH"
	TypeTradeDeal  InitiativeType = "TRADE_DEAL"
	TypeInnovation InitiativeType = "INNOVATION"
	TypePolicy     InitiativeType = "POLICY"
	TypeSecurity   InitiativeType = "SECURITY"
	TypeGreen      InitiativeType = "GREEN"
)

// Initiative represents a policy proposal for a city.
type Initiative struct {
	ID          string         `json:"id"`
	Type        InitiativeType `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Cost        int64          `json:"cost"`
	Effects     city.InitiativeEffects `json:"effects"`
	Duration    int            `json:"duration"` // heartbeats
}

// Generator creates initiative proposals using Claude AI.
type Generator struct {
	client *Client
}

// NewGenerator creates a new initiative generator.
func NewGenerator(client *Client) *Generator {
	return &Generator{client: client}
}

// GenerateForCity asks Claude to produce 3-4 contextually relevant initiative proposals.
func (g *Generator) GenerateForCity(ctx context.Context, c *city.City, allCities []*city.City, round int) ([]Initiative, error) {
	prompt := buildPrompt(c, allCities, round)
	raw, err := g.client.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("claude api: %w", err)
	}
	return parseInitiatives(raw)
}

func buildPrompt(c *city.City, allCities []*city.City, round int) string {
	// Build world context
	otherCities := ""
	for _, oc := range allCities {
		if oc.ID != c.ID {
			otherCities += fmt.Sprintf("  - %s: pop %d, happiness %.0f, treasury %d, products: %s\n",
				oc.Name, oc.Population.Total, oc.Happiness, oc.Treasury,
				strings.Join(oc.Products, ", "))
		}
	}
	if otherCities == "" {
		otherCities = "  (no other cities yet)\n"
	}

	activePolicies := ""
	for _, p := range c.ActivePolicies {
		activePolicies += fmt.Sprintf("  - %s (expires round %d)\n", p.Title, p.ExpiresAt)
	}
	if activePolicies == "" {
		activePolicies = "  (none)\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`You are the Game Coordinator AI for "Cities", a multiplayer social city simulator.

GAME ROUND: %d

YOUR CITY STATE:
  Name: %s
  Population: %d total (workers: %d, families: %d, entrepreneurs: %d, students: %d, homeless: %d)
  Happiness: %.1f/100
  Treasury: %d Citycoins
  Tax Rate: %.1f%%
  Buildings: %s
  Products: %s
  Attractiveness Score: %.1f/100
  Stats:
    - Unemployment: %d%%
    - Crime Rate: %d%%
    - Education Level: %d/100
    - Health Level: %d/100
    - GDP: %d Citycoins

ACTIVE POLICIES:
%s
OTHER CITIES IN THE WORLD:
%s
TASK:
Generate exactly 3 to 4 distinct initiative proposals for the mayor of %s to choose from.
Proposals must be:
1. Contextually relevant to the city's current state and challenges
2. A mix of short-term and long-term impacts
3. Varied in type (don't repeat the same type twice)
4. Balanced: include some affordable options and some ambitious ones
5. Realistic given the city's treasury

Respond with ONLY a valid JSON array. No markdown, no explanation, just the JSON.
Each initiative must have this exact structure:
[
  {
    "id": "unique-id-1",
    "type": "HOUSING|INDUSTRY|EDUCATION|HEALTH|TAX_CHANGE|TRADE_DEAL|INNOVATION|POLICY|SECURITY|GREEN",
    "title": "Short title (max 50 chars)",
    "description": "1-2 sentence description of what the initiative does and why",
    "cost": 2000,
    "effects": {
      "population_delta": 100,
      "happiness_delta": 5.0,
      "treasury_delta": -2000,
      "jobs_delta": 50,
      "new_products": [],
      "new_buildings": ["affordable_housing"],
      "faction_deltas": {"workers": 5, "business": -3, "families": 2, "greens": 0}, // political impact on each faction (-20 to +20)
      "description": "Short summary of the effect"
    },
    "duration": 3
  }
]

Valid building types for new_buildings: house, affordable_housing, school, university, hospital, factory, market, park, police, lab, port
Valid product IDs for new_products: food, grain, fish, meat, fruit, beverages, wood, stone, metal, cement, glass, chemicals, clothing, furniture, tools, electronics, vehicles, appliances, education, healthcare, transport, finance, consulting, insurance, coal, oil, solar, wind, nuclear, art, entertainment, tourism, jewelry, gourmet, software, ai_services, biotech, robotics, cleantech, paper, plastic, steel, textiles, medicine, security, logistics, agriculture, luxury_housing, data
`,
		round,
		c.Name,
		c.Population.Total, c.Population.Workers, c.Population.Families,
		c.Population.Entrepreneurs, c.Population.Students, c.Population.Homeless,
		c.Happiness,
		c.Treasury,
		c.TaxRate,
		buildingList(c.Buildings),
		strings.Join(c.Products, ", "),
		c.AttractivenessScore(),
		c.Stats.UnemploymentRate,
		c.Stats.CrimeRate,
		c.Stats.EducationLevel,
		c.Stats.HealthLevel,
		c.Stats.GDP,
		activePolicies,
		otherCities,
		c.Name,
	))

	return sb.String()
}

func buildingList(buildings []city.Building) string {
	if len(buildings) == 0 {
		return "(none)"
	}
	names := make([]string, len(buildings))
	for i, b := range buildings {
		names[i] = b.Name
	}
	return strings.Join(names, ", ")
}

// parseInitiatives extracts Initiative objects from Claude's JSON response.
func parseInitiatives(raw string) ([]Initiative, error) {
	// Try to find the JSON array in the response
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response: %s", raw)
	}
	jsonStr := raw[start : end+1]

	// Unmarshal into a flexible structure first
	var raw_items []struct {
		ID          string         `json:"id"`
		Type        string         `json:"type"`
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Cost        int64          `json:"cost"`
		Duration    int            `json:"duration"`
		Effects     struct {
			PopulationDelta int            `json:"population_delta"`
			HappinessDelta  float64        `json:"happiness_delta"`
			TreasuryDelta   int64          `json:"treasury_delta"`
			JobsDelta       int            `json:"jobs_delta"`
			NewProducts     []string       `json:"new_products"`
			NewBuildings    []string       `json:"new_buildings"`
			FactionDeltas   map[string]int `json:"faction_deltas"`
			Description     string         `json:"description"`
		} `json:"effects"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw_items); err != nil {
		return nil, fmt.Errorf("parse initiatives JSON: %w\nraw: %s", err, jsonStr)
	}

	var initiatives []Initiative
	for _, ri := range raw_items {
		buildings := make([]city.BuildingType, 0, len(ri.Effects.NewBuildings))
		for _, b := range ri.Effects.NewBuildings {
			buildings = append(buildings, city.BuildingType(b))
		}

		if ri.ID == "" {
			ri.ID = fmt.Sprintf("init-%d", len(initiatives)+1)
		}

		initiatives = append(initiatives, Initiative{
			ID:          ri.ID,
			Type:        InitiativeType(ri.Type),
			Title:       ri.Title,
			Description: ri.Description,
			Cost:        ri.Cost,
			Duration:    ri.Duration,
			Effects: city.InitiativeEffects{
				PopulationDelta: ri.Effects.PopulationDelta,
				HappinessDelta:  ri.Effects.HappinessDelta,
				TreasuryDelta:   ri.Effects.TreasuryDelta,
				JobsDelta:       ri.Effects.JobsDelta,
				NewProducts:     ri.Effects.NewProducts,
				NewBuildings:    buildings,
				FactionDeltas:   ri.Effects.FactionDeltas,
				Description:     ri.Effects.Description,
			},
		})
	}

	if len(initiatives) == 0 {
		return nil, fmt.Errorf("no valid initiatives parsed")
	}

	return initiatives, nil
}

// FallbackInitiatives returns hard-coded initiatives when AI is unavailable.
func FallbackInitiatives(c *city.City) []Initiative {
	initiatives := []Initiative{
		{
			ID: "fallback-1", Type: TypeHousing,
			Title:       "Build Affordable Housing Complex",
			Description: "Construct a new affordable housing block to attract families and reduce homelessness.",
			Cost:        3000,
			Duration:    3,
			Effects: city.InitiativeEffects{
				PopulationDelta: 80, HappinessDelta: 5, TreasuryDelta: -3000,
				NewBuildings: []city.BuildingType{city.BuildingAffordableHousing},
				Description:  "Attracts 80 residents, +5 happiness",
			},
		},
		{
			ID: "fallback-2", Type: TypeIndustry,
			Title:       "Open New Factory District",
			Description: "Develop an industrial zone to create jobs and new production capacity.",
			Cost:        5000,
			Duration:    5,
			Effects: city.InitiativeEffects{
				PopulationDelta: 50, HappinessDelta: -2, TreasuryDelta: -5000, JobsDelta: 100,
				NewBuildings: []city.BuildingType{city.BuildingFactory},
				NewProducts:  []string{"metal", "clothing"},
				Description:  "+100 jobs, new products: metal, clothing",
			},
		},
		{
			ID: "fallback-3", Type: TypeTaxChange,
			Title:       "Reduce Tax Rate by 5%",
			Description: "Lower taxes to attract entrepreneurs and boost economic activity.",
			Cost:        0,
			Duration:    4,
			Effects: city.InitiativeEffects{
				PopulationDelta: 20, HappinessDelta: 8, TreasuryDelta: 0,
				Description: "Attracts entrepreneurs, +8 happiness, reduced tax income",
			},
		},
		{
			ID: "fallback-4", Type: TypeEducation,
			Title:       "Fund Public School Expansion",
			Description: "Expand educational facilities to improve the city's education level.",
			Cost:        2000,
			Duration:    4,
			Effects: city.InitiativeEffects{
				PopulationDelta: 30, HappinessDelta: 6, TreasuryDelta: -2000,
				NewBuildings: []city.BuildingType{city.BuildingSchool},
				Description:  "+6 happiness, education level +10",
			},
		},
	}

	// Filter to ones the city can afford
	affordable := []Initiative{}
	for _, init := range initiatives {
		if init.Cost <= c.Treasury {
			affordable = append(affordable, init)
		}
	}
	if len(affordable) >= 3 {
		return affordable
	}
	return initiatives // return all if none are affordable
}

// ensure trade package is not unused
var _ = trade.Catalog
