# Cities Social Simulator — UI Overhaul & Trade Market Design
**Date:** 2026-03-12

---

## Context

The existing web client runs in an 800×400 Phaser canvas, shows a minimal HUD, and has no trade UI despite a fully-implemented backend trade engine. Numbers update statically with no visual feedback. Each session requires re-entering player/city name and IDs.

This overhaul makes the game fill a full 1080p screen, adds visual dynamism (animated counters, population arrows), persists session tokens in localStorage, exposes the complete resource catalog from the Blueprint, and wires the existing trade engine to a new market UI.

---

## 1. Layout — CSS Grid Full-Screen (1920×1080)

**Architecture:** HTML/CSS Grid owns the layout; Phaser renders only the city visual.

```
┌──────────────────────────────────── HUD (fixed top, ~48px) ────────┐
│  🏙️ Nova City  👥 1,234 ↑+34  😊 72%  💰 45,000¢  📊 Round 7    │
└────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────┬── RESOURCES PANEL (300px) ──┐
│                                      │  Tier 1 ─────────────────   │
│  PHASER CITY CANVAS                  │  🪵 Madera    120  ↑        │
│  calc(100vw - 300px) × 55vh          │  🪨 Piedra     45  —        │
│  min 900×500                         │  ⛓️ Hierro      0  ↓ (dim) │
│                                      │  💧 Agua      200  ↑        │
│                                      │  ... (10 Tier-1 items)      │
│                                      │  Tier 2 ─────────────────   │
│                                      │  🍞 Pan        54  ↑        │
│                                      │  ⚡ Energía    99  ↑        │
│                                      │  ... (20 Tier-2 items)      │
├──── MY PRODUCTS (50%) ───────────────┴──── TRADE MARKET (50%) ─────┤
│  Producto   Stock  Precio sugerido  Acción                          │
│  🍞 Pan       54      18¢          [VENDER]                        │
│  ⚡ Energía   99      22¢          [VENDER]                        │
│ ─────────────────────────────────────────────────────────────────  │
│  ÓRDENES ACTIVAS           │  Ciudad      Producto  Qty  Precio    │
│  🍞 Pan×50 → 18¢ [CANCEL] │  IronHold   ⛓️ Hierro  200   8¢ [BUY]│
└────────────────────────────────────────────────────────────────────┘
```

**Grid structure (CSS):**
- `#app-grid`: `display: grid; grid-template-rows: 48px 55vh 1fr; height: 100vh;`
- Middle row: `grid-template-columns: 1fr 300px;` (canvas | resources)
- Bottom row: `grid-template-columns: 1fr 1fr;` (my-products | market)

**Responsive (<768px):**
- Resources panel moves below canvas (stacks vertically)
- Bottom panels stack vertically
- Canvas = 100vw × 40vh

**Files changed:** `web/index.html` (full restructure)

---

## 2. Token Persistence (localStorage)

**On registration success** → save to `localStorage`:
```json
{
  "playerID": "...",
  "cityID": "...",
  "playerName": "Alexandra",
  "cityName": "Nova City"
}
```
Key: `cities_session`

**On page load:**
- If `cities_session` exists → show "BIENVENIDO DE VUELTA" screen with player/city name + **CONTINUAR** button (auto-connects, skips login) + **NUEVA CIUDAD** link (clears session)
- If not → show normal login form

**File changed:** `web/src/main.js` — `startGame()`, new `checkSavedSession()`, new `resumeSession()`

---

## 3. Animated Counters & Population Arrows

**Counter animation:**
- Before each `city_update`, snapshot current displayed values
- On update: tween each numeric HUD value from old→new over 600ms using `requestAnimationFrame`
- Flash yellow (`#FFD700`) for 300ms when value changes

**Arrow + delta display:**
- Track `prevCity` object (last heartbeat's city state)
- Compare `city.population.total` vs `prevCity.population.total`
- Show `↑+34` (green `#00FF88`) or `↓-11` (red `#FF4444`) or `—` (gray)
- **Persists until next heartbeat** (no timeout) — replaced when next `city_update` arrives
- Applies to: population, treasury, happiness

**File changed:** `web/src/main.js` — `updateHUD()` extended, new `animateCounter()` helper

---

## 4. Resources Panel (Tier 1 & 2)

Full resource catalog from Blueprint, hardcoded as a display map:

```js
const RESOURCE_CATALOG = [
  // Tier 1
  { key: 'wood',      emoji: '🪵', name: 'Madera',   tier: 1 },
  { key: 'stone',     emoji: '🪨', name: 'Piedra',   tier: 1 },
  { key: 'iron',      emoji: '⛓️', name: 'Hierro',   tier: 1 },
  { key: 'copper',    emoji: '🥉', name: 'Cobre',    tier: 1 },
  { key: 'silicon',   emoji: '⏳', name: 'Silicio',  tier: 1 },
  { key: 'water',     emoji: '💧', name: 'Agua',     tier: 1 },
  { key: 'wheat',     emoji: '🌾', name: 'Trigo',    tier: 1 },
  { key: 'oil',       emoji: '🛢️', name: 'Petróleo', tier: 1 },
  { key: 'wool',      emoji: '🐑', name: 'Lana',     tier: 1 },
  { key: 'rubber',    emoji: '🌳', name: 'Caucho',   tier: 1 },
  // Tier 2 — Goods
  { key: 'steel_beams', emoji: '🏗️', name: 'Vigas',     tier: 2 },
  { key: 'bricks',      emoji: '🧱', name: 'Ladrillos', tier: 2 },
  { key: 'tools',       emoji: '🛠️', name: 'Herramienta',tier:2 },
  { key: 'wiring',      emoji: '🔌', name: 'Cableado',  tier: 2 },
  { key: 'bread',       emoji: '🍞', name: 'Pan',        tier: 2 },
  { key: 'clothing',    emoji: '👕', name: 'Ropa',       tier: 2 },
  { key: 'gasoline',    emoji: '⛽', name: 'Gasolina',   tier: 2 },
  { key: 'furniture',   emoji: '🪑', name: 'Muebles',    tier: 2 },
  { key: 'tires',       emoji: '🛞', name: 'Neumáticos', tier: 2 },
  { key: 'glass',       emoji: '🍷', name: 'Vidrio',     tier: 2 },
  // Tier 2 — Services
  { key: 'energy',      emoji: '⚡', name: 'Energía',    tier: 2 },
  { key: 'waste',       emoji: '🗑️', name: 'Basura',     tier: 2 },
  { key: 'security',    emoji: '👮', name: 'Seguridad',  tier: 2 },
  { key: 'education',   emoji: '🎒', name: 'Educación',  tier: 2 },
  { key: 'health',      emoji: '🚑', name: 'Salud',      tier: 2 },
  { key: 'transport',   emoji: '🚌', name: 'Transporte', tier: 2 },
  { key: 'entertainment',emoji:'📻', name: 'Radio',      tier: 2 },
  { key: 'logistics',   emoji: '📦', name: 'Logística',  tier: 2 },
  { key: 'maintenance', emoji: '🧹', name: 'Mantenimiento',tier:2},
  { key: 'water_treatment',emoji:'🚽',name:'Agua Trat.', tier: 2 },
];
```

- Resources with qty=0 rendered dim (opacity 0.4)
- Arrows per resource: same persist-till-heartbeat logic as population
- Panel scrolls vertically if content overflows

**File changed:** `web/index.html` (panel HTML), `web/src/main.js` (`updateResourcesPanel()`)

---

## 5. Trade Market UI

### My Products Panel (bottom-left)

Lists resources your city has in stock > 0. Shows suggested sell price:

```
suggested_price = base_price[product] × clamp(200 / stock, 0.5, 3.0)
```
(High stock → price drops toward 50%; low stock → price rises to 3×)

Click **[VENDER]** → modal dialog: enter quantity + confirm price → calls `POST /api/trade/orders`.

Shows active sell orders you've placed with **[CANCELAR]** (not yet implemented in backend, UI shows it grayed out with tooltip).

### Trade Market Panel (bottom-right)

- Shows all active orders from `GET /api/trade/orders` (polled every 30s + on each city_update)
- Columns: City name | Emoji + Product | Qty available | Price/unit | [COMPRAR] button
- Click **[COMPRAR]** → calls `POST /api/trade/orders` with `side: "buy"` matching the sell order
- Orders settled automatically at next heartbeat by existing `trade.SettleAll()`

### Backend Changes

**`internal/coordinator/server.go`** — add 2 handlers:

```go
// GET /api/trade/orders
// Returns all pending orders from trade engine
func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
    orders := s.tradeEngine.ListPendingOrders()
    json.NewEncoder(w).Encode(orders)
}

// POST /api/trade/orders
// Body: { city_id, product, side, quantity, price }
func (s *Server) handlePlaceOrder(w http.ResponseWriter, r *http.Request) { ... }
```

**`internal/trade/engine.go`** — add `ListPendingOrders() []Order` method (engine already has `orders []Order` slice).

**File changed:** `internal/coordinator/server.go`, `internal/trade/engine.go`

---

## 6. Bug Fixes (while we're here)

- **`main.js:184`**: `gameState.currentHeartbeatID` → `currentHeartbeat.id` (heartbeat_id never set)
- **`population.go:265`**: `recomputeTotals` doesn't count Founders in total — fix by adding `p.Founders` to sum

---

## 7. Phaser Canvas Resize

`web/src/main.js` `initPhaser()`:
- Width: `window.innerWidth - 300` (leave room for resources panel)
- Height: `Math.floor(window.innerHeight * 0.55)`
- Add `scale.on('resize', ...)` handler for responsive reflow

`web/src/scenes/GameScene.js`:
- All `W`/`H` references already use `this.scale.width/height` — no changes needed
- Remove hardcoded `Math.min(window.innerWidth, 800)` cap from `main.js`

---

## 8. Verification

1. `docker-compose up` → open `http://localhost:8080`
2. Register → token saved to localStorage; refresh → "Bienvenido de vuelta" shown
3. Wait for heartbeat (or trigger manually via `POST /api/debug/tick`) → population arrows appear, persist until next heartbeat
4. Confirm numbers animate from old→new on city_update
5. Place a sell order → appears in "Mis Órdenes Activas"; visible in market panel of another browser tab
6. Click [COMPRAR] in second tab → order matched at next heartbeat
7. Resize browser to <768px → layout stacks correctly
8. All 30 resources visible in right panel; zeros shown dim

---

## Files Changed Summary

| File | Change |
|------|--------|
| `web/index.html` | Full CSS Grid layout, resources panel HTML, market panels HTML |
| `web/src/main.js` | Session persistence, animated counters, arrows, market fetch/place, bug fix |
| `web/src/scenes/GameScene.js` | Canvas resize to full viewport |
| `internal/coordinator/server.go` | Add GET/POST `/api/trade/orders` handlers |
| `internal/trade/engine.go` | Add `ListPendingOrders()` method |
| `internal/city/population.go` | Fix founders count in `recomputeTotals` |
