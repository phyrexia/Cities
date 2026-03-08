package trade

import "github.com/cities/game/internal/city"

// ProductCategory groups related products.
type ProductCategory string

const (
	CategoryTier1Raw      ProductCategory = "tier1_raw"       // extraction
	CategoryTier2Goods    ProductCategory = "tier2_goods"     // manufactured goods
	CategoryTier2Services ProductCategory = "tier2_services"  // services
	CategoryTech          ProductCategory = "tech"            // innovation-unlocked
	CategoryLuxury        ProductCategory = "luxury"
)

// Product represents a tradeable good or service.
type Product struct {
	ID                string
	Name              string
	Emoji             string
	Tier              int             // 1=raw material, 2=manufactured/service
	Category          ProductCategory
	BaseCost          int64
	MarketPrice       int64
	RequiredBuildings []city.BuildingType
	RequiredProducts  []string // must also produce these to unlock
}

// Catalog is the global list of all products and services.
// Designed so no city can produce everything — trade is mandatory.
//
// TIER 1: Raw materials (extraction, no special buildings needed unless noted)
// TIER 2: Manufactured goods and services (require factories, schools, etc.)
var Catalog = []Product{

	// ─── TIER 1: RAW MATERIALS ───────────────────────────────────────────────

	{ID: "wood", Name: "Timber", Emoji: "🪵", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 15, MarketPrice: 22},
	{ID: "stone", Name: "Stone", Emoji: "🪨", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 12, MarketPrice: 18},
	{ID: "iron", Name: "Iron Ore", Emoji: "⛓️", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 30, MarketPrice: 48},
	{ID: "copper", Name: "Copper Ore", Emoji: "🥉", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 35, MarketPrice: 55},
	{ID: "silicon", Name: "Silicon Sand", Emoji: "⏳", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 40, MarketPrice: 65},
	{ID: "water", Name: "Fresh Water", Emoji: "💧", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 5, MarketPrice: 10},
	{ID: "grain", Name: "Grain", Emoji: "🌾", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 8, MarketPrice: 13},
	{ID: "oil", Name: "Crude Oil", Emoji: "🛢️", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 50, MarketPrice: 80},
	{ID: "wool", Name: "Raw Wool", Emoji: "🐑", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 20, MarketPrice: 32},
	{ID: "rubber", Name: "Natural Rubber", Emoji: "🌳", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 25, MarketPrice: 40},
	{ID: "coal", Name: "Coal", Emoji: "⛏️", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 18, MarketPrice: 28},
	{ID: "fish", Name: "Fresh Fish", Emoji: "🐟", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 10, MarketPrice: 16},
	{ID: "fruit", Name: "Fruits & Vegetables", Emoji: "🍎", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 10, MarketPrice: 16},
	{ID: "clay", Name: "Clay", Emoji: "🏺", Tier: 1, Category: CategoryTier1Raw,
		BaseCost: 8, MarketPrice: 13},

	// ─── TIER 2: MANUFACTURED GOODS ─────────────────────────────────────────

	{ID: "steel_beam", Name: "Steel Beams", Emoji: "🏗️", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 80, MarketPrice: 130,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"iron", "coal"}},
	{ID: "brick", Name: "Bricks", Emoji: "🧱", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 25, MarketPrice: 40,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"clay", "stone"}},
	{ID: "tools", Name: "Industrial Tools", Emoji: "🛠️", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 50, MarketPrice: 80,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"iron"}},
	{ID: "wiring", Name: "Electrical Wiring", Emoji: "🔌", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 60, MarketPrice: 95,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"copper"}},
	{ID: "bread", Name: "Bread & Flour", Emoji: "🍞", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 12, MarketPrice: 20,
		RequiredBuildings: []city.BuildingType{city.BuildingMarket},
		RequiredProducts:  []string{"grain", "water"}},
	{ID: "food", Name: "Processed Food", Emoji: "🥫", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 18, MarketPrice: 30,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"grain"}},
	{ID: "clothing", Name: "Clothing", Emoji: "👕", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 30, MarketPrice: 50,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"wool"}},
	{ID: "gasoline", Name: "Gasoline", Emoji: "⛽", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 60, MarketPrice: 95,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"oil"}},
	{ID: "furniture", Name: "Furniture", Emoji: "🪑", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 55, MarketPrice: 90,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"wood"}},
	{ID: "tires", Name: "Tires", Emoji: "🛞", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 45, MarketPrice: 72,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"rubber"}},
	{ID: "glass", Name: "Glass Products", Emoji: "🍷", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 35, MarketPrice: 55,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"silicon", "stone"}},
	{ID: "medicine", Name: "Medicine", Emoji: "💊", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 90, MarketPrice: 150,
		RequiredBuildings: []city.BuildingType{city.BuildingHospital, city.BuildingLab},
		RequiredProducts:  []string{"water"}},
	{ID: "electronics", Name: "Consumer Electronics", Emoji: "📱", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 100, MarketPrice: 170,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory, city.BuildingLab},
		RequiredProducts:  []string{"silicon", "copper", "wiring"}},
	{ID: "vehicles", Name: "Vehicles", Emoji: "🚗", Tier: 2, Category: CategoryTier2Goods,
		BaseCost: 200, MarketPrice: 340,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory, city.BuildingLab},
		RequiredProducts:  []string{"steel_beam", "tires", "wiring"}},

	// ─── TIER 2: SERVICES ────────────────────────────────────────────────────

	{ID: "energy", Name: "Electricity", Emoji: "⚡", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 40, MarketPrice: 65,
		RequiredBuildings: []city.BuildingType{city.BuildingPowerPlant},
		RequiredProducts:  []string{"coal"}},
	{ID: "waste_mgmt", Name: "Waste Management", Emoji: "🗑️", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 30, MarketPrice: 50,
		RequiredBuildings: []city.BuildingType{city.BuildingWasteManagement}},
	{ID: "security_svc", Name: "Security Services", Emoji: "👮", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 50, MarketPrice: 80,
		RequiredBuildings: []city.BuildingType{city.BuildingPolice}},
	{ID: "education_svc", Name: "Education", Emoji: "🎒", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 60, MarketPrice: 100,
		RequiredBuildings: []city.BuildingType{city.BuildingSchool}},
	{ID: "healthcare_svc", Name: "Healthcare", Emoji: "🚑", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 80, MarketPrice: 130,
		RequiredBuildings: []city.BuildingType{city.BuildingHospital}},
	{ID: "transit", Name: "Public Transit", Emoji: "🚌", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 45, MarketPrice: 72,
		RequiredBuildings: []city.BuildingType{city.BuildingMarket},
		RequiredProducts:  []string{"gasoline"}},
	{ID: "entertainment", Name: "Entertainment/Radio", Emoji: "📻", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 35, MarketPrice: 60},
	{ID: "logistics", Name: "Logistics & Shipping", Emoji: "📦", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 40, MarketPrice: 65,
		RequiredBuildings: []city.BuildingType{city.BuildingPort}},
	{ID: "maintenance", Name: "Urban Maintenance", Emoji: "🧹", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 30, MarketPrice: 50},
	{ID: "water_treatment", Name: "Water Treatment", Emoji: "🚽", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 35, MarketPrice: 55,
		RequiredBuildings: []city.BuildingType{city.BuildingWaterTreatment},
		RequiredProducts:  []string{"water"}},
	{ID: "finance_svc", Name: "Financial Services", Emoji: "🏦", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 50, MarketPrice: 85,
		RequiredBuildings: []city.BuildingType{city.BuildingMarket}},
	{ID: "consulting", Name: "Business Consulting", Emoji: "💼", Tier: 2, Category: CategoryTier2Services,
		BaseCost: 70, MarketPrice: 115,
		RequiredBuildings: []city.BuildingType{city.BuildingUniversity}},

	// ─── LUXURY ─────────────────────────────────────────────────────────────

	{ID: "art", Name: "Art & Culture", Emoji: "🎨", Tier: 2, Category: CategoryLuxury,
		BaseCost: 100, MarketPrice: 200,
		RequiredBuildings: []city.BuildingType{city.BuildingUniversity}},
	{ID: "tourism", Name: "Tourism", Emoji: "🏖️", Tier: 2, Category: CategoryLuxury,
		BaseCost: 80, MarketPrice: 160,
		RequiredBuildings: []city.BuildingType{city.BuildingPark, city.BuildingMarket}},
	{ID: "jewelry", Name: "Jewelry", Emoji: "💎", Tier: 2, Category: CategoryLuxury,
		BaseCost: 200, MarketPrice: 400,
		RequiredBuildings: []city.BuildingType{city.BuildingFactory},
		RequiredProducts:  []string{"copper", "glass"}},
	{ID: "gourmet", Name: "Gourmet Food", Emoji: "🍽️", Tier: 2, Category: CategoryLuxury,
		BaseCost: 90, MarketPrice: 160,
		RequiredBuildings: []city.BuildingType{city.BuildingMarket},
		RequiredProducts:  []string{"fish", "fruit", "bread"}},

	// ─── TECH (innovation-unlocked) ──────────────────────────────────────────

	{ID: "software", Name: "Software", Emoji: "💾", Tier: 2, Category: CategoryTech,
		BaseCost: 120, MarketPrice: 280,
		RequiredBuildings: []city.BuildingType{city.BuildingLab, city.BuildingUniversity},
		RequiredProducts:  []string{"electronics"}},
	{ID: "ai_services", Name: "AI Services", Emoji: "🤖", Tier: 2, Category: CategoryTech,
		BaseCost: 250, MarketPrice: 600,
		RequiredBuildings: []city.BuildingType{city.BuildingLab, city.BuildingUniversity},
		RequiredProducts:  []string{"software", "energy"}},
	{ID: "biotech", Name: "Biotechnology", Emoji: "🧬", Tier: 2, Category: CategoryTech,
		BaseCost: 300, MarketPrice: 700,
		RequiredBuildings: []city.BuildingType{city.BuildingLab, city.BuildingHospital, city.BuildingUniversity},
		RequiredProducts:  []string{"medicine"}},
	{ID: "robotics", Name: "Robotics", Emoji: "🦾", Tier: 2, Category: CategoryTech,
		BaseCost: 280, MarketPrice: 650,
		RequiredBuildings: []city.BuildingType{city.BuildingLab, city.BuildingFactory},
		RequiredProducts:  []string{"electronics", "steel_beam", "software"}},
	{ID: "cleantech", Name: "Clean Technology", Emoji: "♻️", Tier: 2, Category: CategoryTech,
		BaseCost: 200, MarketPrice: 450,
		RequiredBuildings: []city.BuildingType{city.BuildingLab},
		RequiredProducts:  []string{"energy"}},
	{ID: "solar_panels", Name: "Solar Panels", Emoji: "☀️", Tier: 2, Category: CategoryTech,
		BaseCost: 150, MarketPrice: 320,
		RequiredBuildings: []city.BuildingType{city.BuildingLab, city.BuildingFactory},
		RequiredProducts:  []string{"silicon", "wiring"}},
}

// GetByID returns a product by ID.
func GetByID(id string) *Product {
	for i := range Catalog {
		if Catalog[i].ID == id { return &Catalog[i] }
	}
	return nil
}

// CanProduce checks if a city has the required buildings AND input products.
func CanProduce(p *Product, c interface {
	HasBuilding(city.BuildingType) bool
	GetProducts() []string
}) bool {
	for _, req := range p.RequiredBuildings {
		if !c.HasBuilding(req) { return false }
	}
	cityProducts := c.GetProducts()
	for _, reqProd := range p.RequiredProducts {
		found := false
		for _, cp := range cityProducts {
			if cp == reqProd { found = true; break }
		}
		if !found { return false }
	}
	return true
}

// Tier1Products returns all Tier 1 raw material product IDs.
func Tier1Products() []string {
	var ids []string
	for _, p := range Catalog {
		if p.Tier == 1 { ids = append(ids, p.ID) }
	}
	return ids
}

// Tier2Products returns all Tier 2 product IDs.
func Tier2Products() []string {
	var ids []string
	for _, p := range Catalog {
		if p.Tier == 2 { ids = append(ids, p.ID) }
	}
	return ids
}
