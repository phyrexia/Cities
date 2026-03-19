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
	AllEventDefs = append(AllEventDefs, chainEvents()...)
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
	if c.Happiness < 0 {
		c.Happiness = 0
	}
	if c.Happiness > 100 {
		c.Happiness = 100
	}

	for k, v := range chosen.ResourceDeltas {
		if c.Resources == nil {
			c.Resources = make(map[string]int)
		}
		c.Resources[k] += v
		if c.Resources[k] < 0 {
			c.Resources[k] = 0
		}
	}
	for k, v := range chosen.FactionDeltas {
		switch k {
		case "workers":
			c.Factions.Workers = clampFac(c.Factions.Workers + v)
		case "business":
			c.Factions.Business = clampFac(c.Factions.Business + v)
		case "families":
			c.Factions.Families = clampFac(c.Factions.Families + v)
		case "greens":
			if c.Factions.GreensActive {
				c.Factions.Greens = clampFac(c.Factions.Greens + v)
			}
		}
	}
	for k, v := range chosen.StatDeltas {
		switch k {
		case "crime_rate":
			c.Stats.CrimeRate = clamp(c.Stats.CrimeRate+v, 0, 100)
		case "health_level":
			c.Stats.HealthLevel = clamp(c.Stats.HealthLevel+v, 0, 100)
		case "education_level":
			c.Stats.EducationLevel = clamp(c.Stats.EducationLevel+v, 0, 100)
		case "pollution_level":
			c.Stats.PollutionLevel = clamp(c.Stats.PollutionLevel+v, 0, 100)
		case "innovation_index":
			c.Stats.InnovationIndex = clamp(c.Stats.InnovationIndex+v, 0, 100)
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
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ─── Crisis Events ───────────────────────────────────────────────────────────

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
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.HasBuilding(city.BuildingFactory) && c.Stats.PollutionLevel > 40
			},
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
				factoryLvl := 0
				for _, b := range c.Buildings {
					if b.Type == city.BuildingFactory && b.Level > factoryLvl {
						factoryLvl = b.Level
					}
				}
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
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.Resources != nil && c.Resources["energy"] < 5 && len(c.Buildings) > 4
			},
			Options: []EventOptionDef{
				{Title: "Generadores de emergencia", Description: "Alquilar generadores", TreasuryDelta: -4000, ResourceDeltas: map[string]int{"energy": 30}, FactionDeltas: map[string]int{"families": 5}},
				{Title: "Cortes rotativos", Description: "Energía por turnos", FactionDeltas: map[string]int{"business": -10, "families": -5}, ResourceDeltas: map[string]int{"energy": 15}},
				{Title: "Priorizar hospitales", Description: "Solo servicios esenciales", FactionDeltas: map[string]int{"families": 10, "business": -15}, ResourceDeltas: map[string]int{"energy": 10}, ChainEventID: "looting", ChainDelay: 2},
			}},
		{ID: "flood", Category: "crisis", Title: "Inundación", Description: "Lluvias torrenciales inundan la ciudad. Daños generalizados.", Cooldown: 12, Urgency: 2, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return !c.HasBuilding(city.BuildingWaterTreatment) && rand.Intn(100) < 8
			},
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

// ─── Opportunity Events ──────────────────────────────────────────────────────

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
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.HasBuilding(city.BuildingLab) && c.Resources != nil && c.Resources["knowledge"] > 30
			},
			Options: []EventOptionDef{
				{Title: "Patentar", Description: "Monopolizar los beneficios", FactionDeltas: map[string]int{"business": 15, "workers": -5}, TreasuryDelta: 6000, StatDeltas: map[string]int{"innovation_index": 10}},
				{Title: "Open-source", Description: "Compartir con el mundo", FactionDeltas: map[string]int{"workers": 10, "families": 10, "greens": 5, "business": -5}, StatDeltas: map[string]int{"innovation_index": 15, "education_level": 5}, ChainEventID: "startups_flourish", ChainDelay: 4},
				{Title: "Vender derechos", Description: "Dinero rápido", TreasuryDelta: 10000, FactionDeltas: map[string]int{"business": 5, "workers": -5}},
			}},
		{ID: "tourism_boom", Category: "opportunity", Title: "Boom turístico", Description: "Tu ciudad se vuelve destino popular. Turistas llegan en masa.", Cooldown: 10, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.Happiness > 70 && c.HasBuilding(city.BuildingPark)
			},
			Options: []EventOptionDef{
				{Title: "Invertir infraestructura", Description: "Hoteles y transporte", FactionDeltas: map[string]int{"business": 15, "workers": 10, "greens": -5}, TreasuryDelta: -5000, ChainEventID: "cultural_renaissance", ChainDelay: 5},
				{Title: "Limitar turismo", Description: "Preservar calidad de vida", FactionDeltas: map[string]int{"families": 10, "greens": 10, "business": -10}},
				{Title: "Laissez-faire", Description: "Dejar que el mercado decida", FactionDeltas: map[string]int{"business": 5}, TreasuryDelta: 3000},
			}},
		{ID: "international_fair", Category: "opportunity", Title: "Feria internacional", Description: "Tu mercado atrae atención internacional. Proponen una feria.", Cooldown: 8, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				for _, b := range c.Buildings {
					if b.Type == city.BuildingMarket && b.Level > 2 {
						return true
					}
				}
				return false
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
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.Happiness > 65 && c.HasBuilding(city.BuildingUniversity)
			},
			Options: []EventOptionDef{
				{Title: "Construir galería", Description: "Centro cultural", FactionDeltas: map[string]int{"families": 10, "business": 5}, TreasuryDelta: -3000, HappinessDelta: 5, ChainEventID: "cultural_renaissance", ChainDelay: 4},
				{Title: "Residencia artística", Description: "Programa de arte", FactionDeltas: map[string]int{"families": 5, "workers": 5}, TreasuryDelta: -1000, HappinessDelta: 3},
				{Title: "Nada especial", Description: "Que se instale por su cuenta"},
			}},
		{ID: "mineral_deposit", Category: "opportunity", Title: "Yacimiento mineral", Description: "Se descubre un yacimiento rico cerca de la ciudad.", Cooldown: 20, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.HasBuilding(city.BuildingFactory) && rand.Intn(100) < 5
			},
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
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.HasBuilding(city.BuildingMarket) && c.Resources != nil && c.Resources["food"] > 100
			},
			Options: []EventOptionDef{
				{Title: "Exportar", Description: "Vender el excedente", FactionDeltas: map[string]int{"business": 10}, TreasuryDelta: 4000, ResourceDeltas: map[string]int{"food": -40}},
				{Title: "Almacenar", Description: "Reservas para crisis", FactionDeltas: map[string]int{"families": 10}, ResourceDeltas: map[string]int{"food": 30}},
				{Title: "Festival de cosecha", Description: "Celebrar con el pueblo", FactionDeltas: map[string]int{"workers": 10, "families": 10}, HappinessDelta: 5, ResourceDeltas: map[string]int{"food": -20}},
			}},
		{ID: "space_program", Category: "opportunity", Title: "Programa espacial popular", Description: "Científicos proponen un ambicioso programa de investigación.", Cooldown: 25, Urgency: 5, DefaultOpt: 2,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.HasBuilding(city.BuildingLab) && c.HasBuilding(city.BuildingUniversity) && c.Stats.InnovationIndex > 60
			},
			Options: []EventOptionDef{
				{Title: "Financiar", Description: "Inversión estatal total", FactionDeltas: map[string]int{"families": 15, "business": -5, "workers": 5}, TreasuryDelta: -10000, StatDeltas: map[string]int{"innovation_index": 20}},
				{Title: "Colaborar privados", Description: "Asociación público-privada", FactionDeltas: map[string]int{"business": 10, "families": 10}, TreasuryDelta: -5000, StatDeltas: map[string]int{"innovation_index": 15}},
				{Title: "Solo simbólico", Description: "Apoyo moral nada más", FactionDeltas: map[string]int{"families": -5}, StatDeltas: map[string]int{"innovation_index": 3}},
			}},
	}
}

// ─── Social Events ───────────────────────────────────────────────────────────

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
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.Factions.GreensActive && c.Factions.Greens < 25
			},
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
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return c.Population.Students > 100 && c.Stats.EducationLevel > 60
			},
			Options: []EventOptionDef{
				{Title: "Mentorías", Description: "Programa intergeneracional", FactionDeltas: map[string]int{"families": 10, "workers": 5}, TreasuryDelta: -1000, StatDeltas: map[string]int{"education_level": 3}},
				{Title: "Consejo joven", Description: "Voz en el gobierno", FactionDeltas: map[string]int{"families": 5, "business": -3}, HappinessDelta: 2},
				{Title: "Ignorar tensión", Description: "Se resolverá solo", FactionDeltas: map[string]int{"families": -5}, HappinessDelta: -2},
			}},
	}
}

// ─── World Events ────────────────────────────────────────────────────────────

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
			Trigger: func(c *city.City, ws WorldStateSnapshot) bool {
				return len(c.Products) > 5 && ws.TotalActiveCities > 1
			},
			Options: []EventOptionDef{
				{Title: "Innovar", Description: "Invertir en diferenciación", FactionDeltas: map[string]int{"business": 10, "workers": 5}, TreasuryDelta: -3000, StatDeltas: map[string]int{"innovation_index": 5}},
				{Title: "Bajar precios", Description: "Competir en costos", FactionDeltas: map[string]int{"business": 5, "workers": -10}, TreasuryDelta: -2000},
				{Title: "Diversificar", Description: "Nuevos productos", FactionDeltas: map[string]int{"workers": 5, "business": 5}, TreasuryDelta: -1000},
			}},
		{ID: "alliance_proposed", Category: "world", Title: "Alianza propuesta", Description: "Una ciudad con población similar propone cooperación.", Cooldown: 15, Urgency: 3, DefaultOpt: 2,
			Trigger: func(c *city.City, ws WorldStateSnapshot) bool {
				return ws.TotalActiveCities > 1 && c.Population.Total > 400
			},
			Options: []EventOptionDef{
				{Title: "Aceptar", Description: "Alianza completa", FactionDeltas: map[string]int{"business": 10, "families": 5}, TreasuryDelta: 2000, HappinessDelta: 2},
				{Title: "Contra-proponer", Description: "Términos más favorables", FactionDeltas: map[string]int{"business": 15}, TreasuryDelta: 3000},
				{Title: "Rechazar", Description: "Independencia total", FactionDeltas: map[string]int{"workers": 5}},
			}},
		{ID: "supply_blockade", Category: "world", Title: "Bloqueo de suministros", Description: "Una ruta comercial es bloqueada. Recursos escasean.", Cooldown: 10, Urgency: 3, DefaultOpt: 1,
			Trigger: func(c *city.City, _ WorldStateSnapshot) bool {
				return len(c.TradeRoutes) > 2 && rand.Intn(100) < 10
			},
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

// ─── Chain Follow-up Events ──────────────────────────────────────────────────

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
