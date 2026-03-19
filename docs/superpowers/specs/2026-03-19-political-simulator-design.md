# Cities — Political Simulator Redesign

**Date**: 2026-03-19
**Status**: Approved
**Type**: Feature Design Spec

## Overview

Transform Cities from an economic city-builder into a **social/political simulator** where the core gameplay revolves around decisions, consequences, and faction management. The game is primarily single-player but exists in an asynchronous multiplayer world where other players' actions generate indirect effects (refugees, talent migration, trade competition).

### Design Pillars

1. **Every decision has political cost** — pleasing one faction angers another
2. **Consequences are delayed and chained** — decisions echo 3-5 turns later
3. **The city is alive visually** — Phaser scene reacts passively to all state changes
4. **Failure is real but fair** — cities can collapse after turn 16, but never without warning
5. **The world affects you** — other players' crises and booms ripple into your city

---

## 1. Faction System

### 1.1 Four Factions

| Faction | Population Groups | Care About | Angered By |
|---------|-------------------|------------|------------|
| **Trabajadores** (Workers) | Workers + Homeless | Employment, wages, affordable housing | Low taxes on rich, unregulated factories, social cuts |
| **Empresarios** (Business) | Entrepreneurs | Low taxes, innovation, free trade | Excessive regulation, high taxes, bureaucracy |
| **Familias** (Families) | Families + Students | Education, health, safety, parks | High crime, pollution, lack of schools/hospitals |
| **Ecologistas** (Greens) | Transversal (emerges at education >50) | Low pollution, parks, cleantech | Unregulated factories, no water treatment, deforestation |

### 1.2 Satisfaction Mechanics

- Each faction has a satisfaction score: **0-100**
- Every initiative/decision/event moves 2+ faction scores (often in opposite directions)
- Satisfaction drifts toward 50 slowly if no decisions affect it (regression to mean)

### 1.3 Faction Thresholds

| Satisfaction | State | Effect |
|-------------|-------|--------|
| **>80** | Thriving | Passive bonuses: Workers→productivity, Business→treasury, Families→birth rate, Greens→pollution reduction |
| **50-80** | Content | Normal operation |
| **25-50** | Uneasy | Minor penalties, warning indicators |
| **10-25** | Angry | **Protest event** triggers automatically, emigration of that group |
| **<10** | Revolt | **Huelga/Exodus**, severe penalties. See Section 5.3 for phase-dependent collapse rules. |

### 1.4 Faction Starting Values and Per-Tick Calculation

**Starting values** (new city):
- Workers: 55, Business: 50, Families: 60, Greens: hidden (starts at 50 on emergence)

**Per-tick drift**: each faction drifts 1 point toward 50 per turn if no decision affected it that turn.

**Stat-to-faction mapping (applied each tick):**

| City Stat | Affected Faction | Formula |
|-----------|-----------------|---------|
| `unemployment_rate > 20` | Workers | `-1 per 10% above 20` |
| `unemployment_rate < 10` | Workers | `+1` |
| `tax_rate > 30` | Business | `-1 per 5% above 30` |
| `tax_rate < 15` | Business | `+2` |
| `crime_rate > 40` | Families | `-1 per 10% above 40` |
| `education_level > 60` | Families | `+1` |
| `health_level > 60` | Families | `+1` |
| `pollution_level > 30` | Greens | `-1 per 10% above 30` |
| Has park building | Greens | `+1` |
| Has cleantech product | Greens | `+2` |
| `food surplus > pop/10` | Workers, Families | `+1 each` |
| `treasury < 0` | All factions | `-1 each` |

### 1.5 Happiness ↔ Faction Relationship

`Happiness` remains as an independent aggregate metric. It is computed as:

```
Happiness = (Workers_sat * 0.3 + Business_sat * 0.2 + Families_sat * 0.35 + Greens_sat * 0.15) * modifier
```

Where `modifier` accounts for food/water shortages and treasury health (existing `updateHappiness` logic).

If Greens are not yet active, their weight is redistributed: Workers 0.35, Business 0.25, Families 0.40.

The existing `updateHappiness()` function is replaced by this formula. All existing systems (population growth, emigration, attractiveness) continue to read `Happiness` as before.

### 1.6 Ecologist Emergence

- Ecologistas faction is hidden until city's `education_level > 50`
- Once emerged, they never disappear
- Their satisfaction starts at 50 and immediately begins reacting to pollution, parks, cleantech
- Emergence triggers a one-time event: "Environmental Awareness Movement"

---

## 2. Event Engine

### 2.1 Event Structure

```
Event {
  ID:           string
  Category:     crisis | opportunity | social | world | chain
  Title:        string
  Description:  string (2-3 lines of narrative)
  Trigger:      condition function (city state → bool)
  Cooldown:     int (turns before can repeat)
  Urgency:      int (turns before default option applies)
  Options: []{
    Title:       string
    Description: string
    Effects: {
      FactionDeltas:  map[faction]int
      ResourceDeltas: map[resource]int
      TreasuryDelta:  int64
      HappinessDelta: float64
      StatDeltas:     map[stat]int
      SpawnBuilding:  BuildingType (optional)
      ChainEvent:     EventID (optional, fires after ChainDelay turns)
    }
    ChainDelay: int (turns until ChainEvent fires)
  }
  MaxSimultaneous: 2 (max active events at once)
}
```

### 2.2 Event Catalog (52 events)

#### Crisis Events (12)

| # | Event | Trigger | Options |
|---|-------|---------|---------|
| 1 | Epidemia | health <30 | Cuarentena / Hospital de campaña / Ignorar |
| 2 | Sequía severa | water <15 | Racionamiento / Importar agua / Pozos de emergencia |
| 3 | Ola de crimen | crime >60 | Toque de queda / Más policía / Programas sociales |
| 4 | Incendio industrial | factory + pollution >40 | Evacuar zona / Bomberos voluntarios / Cerrar fábrica |
| 5 | Huelga general | workers satisfaction <20 | Negociar / Ceder demandas / Reprimir |
| 6 | Colapso de puente | pop >800 | Reparar urgente / Desviar tráfico / Reconstruir mejor |
| 7 | Fuga de gas | factory lvl >2, no water_treatment | Evacuación masiva / Contener / Minimizar en medios |
| 8 | Crisis alimentaria | food <10 | Racionamiento / Importar de emergencia / Abrir granjas |
| 9 | Apagón masivo | energy <5 | Generadores de emergencia / Cortes rotativos / Priorizar hospitales |
| 10 | Inundación | no water_treatment, random | Evacuación / Diques improvisados / Pedir ayuda externa |
| 11 | Protestas violentas | any faction <15 | Diálogo / Fuerza policial / Concesiones |
| 12 | Escándalo de corrupción | treasury >50000 | Investigar / Encubrir / Purga de funcionarios |

#### Opportunity Events (12)

| # | Event | Trigger | Options |
|---|-------|---------|---------|
| 13 | Inversor extranjero | innovation >40 | Aceptar condiciones / Negociar / Rechazar |
| 14 | Descubrimiento científico | lab + knowledge >30 | Patentar / Open-source / Vender derechos |
| 15 | Boom turístico | happiness >70, park | Invertir infraestructura / Limitar turismo / Laissez-faire |
| 16 | Feria internacional | market lvl >2 | Organizar feria / Patrocinar / Ignorar |
| 17 | Startup exitosa | entrepreneurs >50 | Incubar más / Tax break / Regular |
| 18 | Donación filantrópica | happiness >60 | Hospital / Escuela / Parque / Vivienda |
| 19 | Artista famoso se muda | happiness >65, university | Construir galería / Residencia artística / Nada |
| 20 | Yacimiento mineral | factory, random | Explotar / Explotar sostenible / Reserva natural |
| 21 | Acuerdo comercial favorable | trade_routes >1 | Exclusividad / Abierto a todos / Rechazar |
| 22 | Ciudad hermana | pop >600 | Aceptar alianza / Proponer términos / Declinar |
| 23 | Cosecha récord | market + food >100 | Exportar / Almacenar / Festival de cosecha |
| 24 | Programa espacial popular | lab + university + innovation >60 | Financiar / Colaborar privados / Solo simbólico |

#### Social Events (10)

| # | Event | Trigger | Options |
|---|-------|---------|---------|
| 25 | Protesta de trabajadores | workers sat <25 | Subir salario mínimo / Mesa de diálogo / Ignorar |
| 26 | Lobby empresarial | business sat <25 | Reducir regulación / Escuchar sin ceder / Rechazar lobby |
| 27 | Marcha de familias | families sat <25 | Más escuelas y parques / Prometer reformas / Desestimar |
| 28 | Bloqueo ecologista | greens sat <25 | Cerrar fábrica contaminante / Plan verde / Desalojar |
| 29 | Festival espontáneo | any faction >80 | Patrocinar / Dejar ser / Cobrar permiso |
| 30 | Movimiento vecinal | families sat >70 | Apoyar con fondos / Dar autonomía / Cooptar |
| 31 | Cooperativa obrera | workers sat >75 | Subsidiar / Facilitar terreno / Ignorar |
| 32 | Hackathon de emprendedores | business sat >70 | Premiar ganadores / Co-invertir / Solo publicidad |
| 33 | Voluntariado masivo | happiness >75 | Canalizar a infraestructura / Medio ambiente / Educación |
| 34 | División generacional | students >100 + education >60 | Mentorías / Consejo joven / Ignorar tensión |

#### World/Asymmetric Events (10)

| # | Event | Trigger | Options |
|---|-------|---------|---------|
| 35 | Ola de refugiados | otra ciudad colapsa | Acoger todos / Acoger algunos / Cerrar fronteras |
| 36 | Fuga de talento | otra ciudad boom económico | Contra-oferta fiscal / Mejorar calidad vida / Dejar ir |
| 37 | Criminales desplazados | otra ciudad crime alto | Reforzar policía / Programas rehabilitación / Nada |
| 38 | Recesión global | timer aleatorio (raro) | Austeridad / Estímulo fiscal / Proteccionismo |
| 39 | Pandemia mundial | timer aleatorio (raro) | Lockdown / Medidas parciales / Negacionismo |
| 40 | Moda cultural de otra ciudad | otra ciudad happiness alto | Importar cultura / Crear propia / Ignorar tendencia |
| 41 | Competencia comercial | otra ciudad mismos productos | Innovar / Bajar precios / Diversificar |
| 42 | Alianza propuesta | otra ciudad pop similar | Aceptar / Contra-proponer / Rechazar |
| 43 | Bloqueo de suministros | trade_routes >2, random | Rutas alternativas / Producción local / Negociar |
| 44 | Migración de científicos | otra ciudad education bajo | Reclutar agresivo / Bienvenida pasiva / No interferir |

#### Event Chains (8 chains, ~16 follow-up events)

| # | Chain | Initial → Consequence |
|---|-------|-----------------------|
| C1 | **Río contaminado** | Fábrica contamina → (ignorar) → Brote enfermedades → (ignorar) → Éxodo de familias |
| C2 | **Boom inmobiliario** | Inversor extranjero → (aceptar) → Gentrificación → Protesta de trabajadores |
| C3 | **Revolución verde** | Bloqueo ecologista → (cerrar fábrica) → Desempleo sube → Huelga de trabajadores |
| C4 | **Edad dorada** | Descubrimiento científico → (open-source) → Startups florecen → Ciudad hermana |
| C5 | **Espiral de crimen** | Apagón masivo → (mal manejado) → Saqueos → Ola de crimen |
| C6 | **Milagro educativo** | Donación filantrópica → (escuela) → Education sube → Ecologistas emergen → Movimiento verde |
| C7 | **Crisis de confianza** | Escándalo corrupción → (encubrir) → Filtración a medios → Protestas violentas |
| C8 | **Renacer cultural** | Artista famoso → (galería) → Boom turístico → Festival internacional |

### 2.3 Event Rules

- Max **2 events active** simultaneously
- Each event has a **cooldown** of 5-10 turns before repeating
- Events are evaluated every heartbeat in priority order: chain > crisis > social > world > opportunity
- During Foundation phase (turns 1-8): only opportunity and positive social events

**Events vs Initiatives priority**: When 1+ events are active, heartbeat initiative proposals are **suppressed**. The player must resolve events before receiving new initiatives. This reinforces the "political cost" pillar — crises demand attention. When no events are active, normal initiative proposals appear.

**Urgency defaults by category:**

| Category | Urgency (turns) | Default option |
|----------|-----------------|----------------|
| Crisis | 2 | Last option (usually worst: "Ignorar", "Nada") |
| Social | 3 | Last option |
| World | 3 | Middle option (moderate response) |
| Opportunity | 5 | Last option (opportunity missed) |
| Chain | 2 | Last option |

### 2.4 Event Decision API

**New WebSocket message types:**

| Type | Direction | Payload |
|------|-----------|---------|
| `event_fired` | Server → Client | `{ event: GameEvent }` |
| `event_decision` | Client → Server | `{ event_id: string, option_id: string }` |
| `event_resolved` | Server → Client | `{ event_id: string, outcome: string, effects_applied: {} }` |
| `chain_triggered` | Server → Client | `{ event: GameEvent, caused_by: string }` |

**New REST endpoint:**
- `POST /api/event/decision` — Submit event decision (alternative to WS for reliability)
  - Body: `{ city_id, event_id, option_id }`
  - Response: `{ ok: true, effects_applied: {} }`

### 2.5 World Event Evaluation

World/asymmetric events (35-44) require cross-city state awareness. Implementation approach:

Before per-city processing each heartbeat, compute a **WorldStateSnapshot**:
```
WorldStateSnapshot {
  AnyCityCollapsed:    bool      // any city entered ruins this round
  AnyCityBoom:         bool      // any city has happiness > 80 AND treasury > 30000
  AnyCityHighCrime:    bool      // any city has crime_rate > 70
  AnyCityLowEducation: bool      // any city has education_level < 20
  TotalActiveCities:   int
  AverageHappiness:    float64
  GlobalRecession:     bool      // random, 2% chance per round after turn 10
  GlobalPandemic:      bool      // random, 1% chance per round after turn 15
}
```

World events reference this snapshot rather than directly querying other cities. This keeps city processing independent.

---

## 3. Phaser Visualization (Passive)

The Phaser canvas is a **living window** into city state. No player interaction — all input happens in panels.

### 3.1 Ambient Layer (always visible)

- **Sky color**: lerps based on happiness (bright blue >70 → grey 30-70 → dark/stormy <30)
- **Day/night cycle**: subtle tint shift based on round number
- **Smoke particles**: from factories, intensity scales with `pollution_level`
- **Nature**: birds fly across sky (count = number of parks), clouds drift
- **Vegetation**: trees/grass density increases with parks + ecologist satisfaction

### 3.2 Citizen Activity Layer

- Citizens walk with **faction colors**: gold=business, blue=workers, green=families, purple=greens
- Count proportional to population (1 sprite per 20 pop, max 50)
- **Walk speed** reflects happiness: fast and varied >70, slow and uniform <30
- Vehicles on road proportional to GDP/economic activity

### 3.3 Reactive Event Visuals

| State | Visual Effect |
|-------|--------------|
| Protest (faction <25) | Group of colored citizens with sign sprites, gathered at building |
| Strike | Factories dark (no smoke), workers standing still |
| Festival (faction >80) | Confetti particles, citizens clustered in groups |
| Epidemic | Red cross flashing over hospital, citizens walk slowly |
| Fire | Orange/red flame particles on affected building |
| Flood | Blue water overlay on ground layer |
| Economic boom | Gold sparkle particles on market/factories |
| High crime | Blue/red police lights flashing on police station |
| Pollution high | Yellow-brown haze overlay, reduced visibility |
| Blackout | Dark overlay, flicker effect |

### 3.4 Permanent Subtle Indicators

- Buildings scale with level (already exists)
- Trash sprites visible if no waste_management building
- Graffiti sprites on walls if crime >50
- Garden patches if families satisfaction >70
- Solar panels on roofs if city has cleantech product

---

## 4. Frontend Layout & Panels

### 4.1 Grid Layout

```
┌─────────────────────────────────────────────────────────┐
│  HUD: City | Pop | Happiness | Treasury | Round         │ 48px
├────────────────────────┬──────────┬─────────────────────┤
│                        │ RECURSOS │  FACCIONES          │
│   PHASER CANVAS        │ +rate/t  │  4 satisfaction     │
│   (passive view)       │ progress │  bars with emoji    │
│                        │ bars     │  state indicator    │
├────────────────────────┴──────────┴─────────────────────┤
│  EVENTO ACTIVO / DECISIÓN                               │
│  [Narrative text + 2-3 option cards]                    │
│  [Each card: faction impact preview + resource impact]  │
│  [Urgency timer bar]                                    │
├──────────────────────────┬──────────────────────────────┤
│  TIMELINE                │  EVENT LOG                   │
│  Last 10 decisions       │  Live feed with icons        │
│  with outcomes + pending │  color-coded by type         │
│  consequences (clock)    │                              │
└──────────────────────────┴──────────────────────────────┘
```

### 4.2 Faction Panel (replaces City Needs)

- 4 horizontal bars, colored per faction
- Reactive emoji: angry (<25) neutral (25-50) content (50-75) happy (>75)
- Tooltip on hover: "Trabajadores: 43% — Preocupados por desempleo"
- Flash animation when satisfaction changes after a decision
- Ecologistas bar hidden until education >50, then fades in with emergence event

### 4.3 Event/Decision Panel (replaces floating heartbeat banner)

- Fixed section in layout, not a popup overlay
- Shows active event with short narrative text (2-3 lines)
- Option cards with impact preview: `Workers +15 | Business -10 | Treasury -2000`
- Visual urgency bar that depletes over turns
- When no event is active, shows normal heartbeat initiative proposals
- When 2 events active: tabs to switch between them

### 4.4 Timeline Panel (new)

- Vertical list of last 10 decisions taken
- Each entry: icon + short title + outcome summary
- Decisions with pending chain consequences marked with clock icon
- When chain consequence arrives, visual connection line appears
- Scrollable, most recent at top

### 4.5 Event Log (improved)

- Remains in corner but with better categorization
- Icons per type: faction moves, resource changes, micro-events, world events
- Color-coded: green=positive, red=negative, gold=important, grey=neutral
- Clickable entries expand to show details (optional enhancement)

---

## 5. Gameplay Loop

### 5.1 Per-Turn Sequence (each heartbeat)

```
1. PRODUCTION    — Resources generated/consumed by buildings + population
2. FACTIONS      — Satisfaction adjusts based on city state
3. PRODUCTS      — Auto-discover products city qualifies for
4. BUILDINGS     — Auto-upgrade if resource/population thresholds met
5. EVENTS        — Evaluate triggers, fire new events
6. MICRO-EVENTS  — 1-3 flavor events with minor effects
7. CHAINS        — Delayed consequences from past decisions arrive
8. WORLD         — Other players' states generate asymmetric effects
9. PRESENT       — Update Phaser, panels, faction bars, event cards
10. PLAYER DECIDES — Choose event option / heartbeat initiative (or timeout → default)
```

### 5.2 Game Phases

| Phase | Turns | Protection | Event Types |
|-------|-------|------------|-------------|
| **Foundation** | 1-8 | Immune to collapse. No crisis events. | Opportunities, positive social, tutorials |
| **Growth** | 9-15 | Immune to collapse. Crises begin. Warnings visible. | All types except world-rare (pandemic, recession) |
| **Maturity** | 16-25 | Collapse possible: 2+ factions <10 OR treasury <-50k. 3 turns grace period. | All types |
| **No Safety Net** | 26+ | Aggressive collapse: 1 faction <5 OR treasury <-30k. 2 turns grace. | All types, higher crisis frequency |

### 5.3 Collapse Mechanics — Definitive Rules

**Collapse is phase-dependent.** The existing `BankruptcyThreshold` constant (-50,000) is replaced by phase-aware logic:

| Phase | Faction Collapse Trigger | Treasury Collapse Trigger | Grace Period |
|-------|-------------------------|--------------------------|--------------|
| Foundation (1-8) | Immune | Immune | N/A |
| Growth (9-15) | Immune | Immune | N/A |
| Maturity (16-25) | 2+ factions <10 for 2+ consecutive turns | treasury < -50,000 | 3 turns |
| No Safety Net (26+) | 1+ faction <5 for 2+ consecutive turns | treasury < -30,000 | 2 turns |

**Collapse flow:**
1. Trigger condition met → city enters **"Collapsing"** state — red banner, special last-chance event fires
2. During grace period: drastic options available (max taxes, close factories, beg external rescue with future cost)
3. If any trigger condition is resolved during grace → city returns to **Crisis** state
4. If grace period expires without recovery → **Ruins**. City becomes claimable by other players
5. Refugees from collapsed city flow to other players' cities (asymmetric world event)

**Collapse is NEVER random** — always preceded by visible faction decline, warning events, and 2-3 turns to react. The "consecutive turns" requirement prevents single-tick spikes from causing collapse.

### 5.4 City Mood (soft state, NOT CityStatus)

City Mood is a **derived label** computed each tick for event frequency and UI display. It does NOT replace `CityStatus` (active/vacation/collapsing/ruins):

| Mood | Condition | Effect |
|------|-----------|--------|
| **Prosperity** | 3+ factions >70, treasury >20k | Opportunity events 2x more likely, population grows fast |
| **Stable** | No faction <30, treasury >0 | Normal event flow |
| **Crisis** | 1+ faction <25 OR treasury <-10k | Crisis events 2x more likely, emigration +50% |
| **Collapsing** | Collapse trigger active | Grace period, drastic options, last chance |

### 5.5 Session Arc (typical 30-50 turn game)

- **Turns 1-5**: Establishment. Resources grow, population arrives, first products unlock. Gentle events (fair, donation).
- **Turns 6-15**: Tensions emerge. Factions start diverging based on decisions. First crisis event. Ecologists may appear if education rises.
- **Turns 16-30**: Complexity. Active chain consequences. World events from other players. Alternating firefighting and opportunity moments.
- **Turns 30+**: Maturity. City defined by governing style. Faction bonuses vs. neglected faction crises. High-level events (space program, alliances).

---

## 6. Technical Implementation Notes

### 6.1 New Data Structures

```go
// Faction satisfaction stored in City
type FactionSatisfaction struct {
    Workers     int `json:"workers"`      // 0-100
    Business    int `json:"business"`     // 0-100
    Families    int `json:"families"`     // 0-100
    Greens      int `json:"greens"`       // 0-100, hidden until education > 50
    GreensActive bool `json:"greens_active"`
}

// GameEvent represents an active event requiring player decision
type GameEvent struct {
    ID          string        `json:"id"`
    EventDefID  string        `json:"event_def_id"`
    Category    string        `json:"category"`
    Title       string        `json:"title"`
    Description string        `json:"description"`
    Options     []EventOption `json:"options"`
    Urgency     int           `json:"urgency"`      // turns remaining
    FiredAt     int           `json:"fired_at"`      // round when event appeared
    DefaultOpt  int           `json:"default_opt"`   // index of default if timeout
}

// EventOption is one choice the player can make
type EventOption struct {
    ID             string            `json:"id"`
    Title          string            `json:"title"`
    Description    string            `json:"description"`
    FactionDeltas  map[string]int    `json:"faction_deltas"`
    ResourceDeltas map[string]int    `json:"resource_deltas"`
    StatDeltas     map[string]int    `json:"stat_deltas"`      // crime_rate, health_level, etc.
    TreasuryDelta  int64             `json:"treasury_delta"`
    HappinessDelta float64           `json:"happiness_delta"`
    SpawnBuilding  string            `json:"spawn_building,omitempty"` // BuildingType to add
    ChainEventID   string            `json:"chain_event_id,omitempty"`
    ChainDelay     int               `json:"chain_delay,omitempty"`
}

// PendingChain tracks a future event triggered by a past decision
type PendingChain struct {
    EventDefID string `json:"event_def_id"`
    FiresAt    int    `json:"fires_at"` // round number
    CausedBy   string `json:"caused_by"` // decision that caused this
}

// DecisionRecord for the timeline
type DecisionRecord struct {
    Round       int    `json:"round"`
    EventTitle  string `json:"event_title"`
    ChoiceTitle string `json:"choice_title"`
    Outcome     string `json:"outcome"`
    HasPending  bool   `json:"has_pending"` // chain consequence pending
}
```

### 6.2 Backend Changes Required

- `internal/city/factions.go` — new file: faction satisfaction engine (per-tick calculation from Section 1.4, drift logic, threshold checks)
- `internal/city/city.go` — add fields to City struct: `Factions FactionSatisfaction`, `ActiveEvents []GameEvent`, `PendingChains []PendingChain`, `DecisionHistory []DecisionRecord`, `Mood string`
- `internal/coordinator/events.go` — new file: event engine (52 event definitions, trigger evaluation, chain resolution, urgency countdown)
- `internal/coordinator/heartbeat.go` — integrate faction tick + event evaluation + world snapshot into game loop
- `internal/city/economy.go` — add `FactionDeltas map[string]int` to `InitiativeEffects` struct; update `ApplyInitiativeEffects()` to apply faction changes
- `internal/ai/initiatives.go` — update AI prompt to include faction impact in generated proposals; update JSON schema for initiative parsing
- `internal/coordinator/server.go` — add `POST /api/event/decision` endpoint; add WS handler for `event_decision` messages

### 6.3 Frontend Changes Required

- `web/src/main.js` — faction panel rendering, event panel with urgency timer, timeline panel, initiative cards show faction impact
- `web/src/api/websocket.js` — add handlers for new message types: `event_fired`, `event_resolved`, `chain_triggered`; add `sendEventDecision()` method
- `web/src/scenes/GameScene.js` — ambient layer (sky color, smoke, nature), reactive event visuals (protests, fires, floods, festivals)
- `web/index.html` — new grid layout with faction panel replacing needs panel, event/decision panel, timeline panel
- `web/src/entities/Building.js` — add visual effect overlays (smoke intensity, fire particles, flood water)

### 6.4 Files Unchanged

- `internal/trade/` — trade system stays as-is for now
- `internal/storage/` — persistence remains out of scope for this iteration
