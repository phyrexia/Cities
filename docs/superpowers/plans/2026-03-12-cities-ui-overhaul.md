# Cities Social Simulator UI Overhaul Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development to implement this plan. Each task creates working, testable changes with frequent commits.

**Goal:** Transform the web client from a minimal 800×400 canvas into a full 1080p responsive UI with animated counters, resources panel, and integrated trade market.

**Architecture:** Separate frontend (HTML/CSS/JS) and backend concerns. Frontend uses CSS Grid for layout with Phaser as an island for city visuals. Backend exposes trade endpoints that frontend polls. Session state lives in localStorage.

**Tech Stack:** Phaser.js 3.80, Vanilla JS (ES6), CSS Grid, localStorage, WebSocket, Go backend with existing trade engine

---

## Chunk 1: Backend Trade Endpoints & Bug Fixes

### Task 1: Add ListPendingOrders method to trade engine

**Files:**
- Modify: `internal/trade/engine.go`

- [ ] **Step 1: Read the current engine structure**

Run: `cat internal/trade/engine.go | head -50`

Verify `orders []Order` field exists and understand the Order struct.

- [ ] **Step 2: Add ListPendingOrders method**

Find the line where other methods are defined (around line 50-100). Add:

```go
// ListPendingOrders returns all currently pending orders
func (e *Engine) ListPendingOrders() []Order {
	e.mu.RLock()
	defer e.mu.RUnlock()
	// Return a copy to prevent external mutation
	result := make([]Order, len(e.orders))
	copy(result, e.orders)
	return result
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/trade/engine.go
git commit -m "feat: add ListPendingOrders method to trade engine"
```

---

### Task 2: Add trade endpoints to server (GET /api/trade/orders)

**Files:**
- Modify: `internal/coordinator/server.go`

- [ ] **Step 1: Find where handlers are registered**

Search for `HandleFunc("/api` in server.go. Identify the mux setup pattern.

- [ ] **Step 2: Add GET /api/trade/orders handler**

Add this function before the server starts (around line 150):

```go
func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orders := s.tradeEngine.ListPendingOrders()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(orders); err != nil {
		log.Printf("[Trade] Error encoding orders: %v", err)
	}
}
```

Then register it in the mux setup:
```go
mux.HandleFunc("/api/trade/orders", s.handleListOrders)
```

- [ ] **Step 3: Test the endpoint manually**

Run: `docker-compose up`

Open another terminal:
```bash
curl -s http://localhost:8080/api/trade/orders | jq .
```

Expected: `[]` (empty array) or array of Order objects.

- [ ] **Step 4: Commit**

```bash
git add internal/coordinator/server.go
git commit -m "feat: add GET /api/trade/orders endpoint"
```

---

### Task 3: Add POST /api/trade/orders handler

**Files:**
- Modify: `internal/coordinator/server.go`

- [ ] **Step 1: Add POST /api/trade/orders handler**

Add this function:

```go
func (s *Server) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CityID   string `json:"city_id"`
		Product  string `json:"product"`
		Side     string `json:"side"` // "buy" or "sell"
		Quantity int    `json:"quantity"`
		Price    int    `json:"price"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create order
	order := trade.Order{
		ID:       uuid.New().String(),
		CityID:   req.CityID,
		Product:  req.Product,
		Side:     req.Side,
		Quantity: req.Quantity,
		Price:    req.Price,
		Status:   "pending",
		CreatedAt: time.Now(),
	}

	// Place order in engine
	s.tradeEngine.PlaceOrder(order)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}
```

Register in mux:
```go
mux.HandleFunc("/api/trade/orders", s.handlePlaceOrder) // will handle both GET and POST via method check
```

Actually, modify the handleListOrders to dispatch:

```go
func (s *Server) handleTradeOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.handleListOrders(w, r)
	} else if r.Method == http.MethodPost {
		s.handlePlaceOrder(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
```

Then register once:
```go
mux.HandleFunc("/api/trade/orders", s.handleTradeOrders)
```

- [ ] **Step 2: Test POST locally**

```bash
curl -X POST http://localhost:8080/api/trade/orders \
  -H "Content-Type: application/json" \
  -d '{"city_id":"test-city","product":"bread","side":"sell","quantity":50,"price":18}'
```

Expected: Returns the created order with ID.

- [ ] **Step 3: Commit**

```bash
git add internal/coordinator/server.go
git commit -m "feat: add POST /api/trade/orders endpoint for placing orders"
```

---

### Task 4: Fix heartbeat_id bug in main.js

**Files:**
- Modify: `web/src/main.js`

- [ ] **Step 1: Find the bug**

Open `web/src/main.js`, find line 184 in `selectProposal()`:

```js
heartbeat_id: gameState.currentHeartbeatID,  // BUG: undefined
```

- [ ] **Step 2: Fix the bug**

Change to:

```js
heartbeat_id: currentHeartbeat.id,  // Correct: use global currentHeartbeat object
```

- [ ] **Step 3: Test**

When a heartbeat arrives, click a proposal → check browser console for the POST payload to verify `heartbeat_id` is now populated.

- [ ] **Step 4: Commit**

```bash
git add web/src/main.js
git commit -m "fix: correct heartbeat_id reference in selectProposal"
```

---

### Task 5: Fix founders count bug in population.go

**Files:**
- Modify: `internal/city/population.go`

- [ ] **Step 1: Find recomputeTotals**

Search for `func (p *PopulationEngine) recomputeTotals()` (around line 265).

Current code:
```go
p.Total = 0
for _, cohort := range p.Cohorts {
    p.Total += cohort.Population
}
```

- [ ] **Step 2: Fix to include Founders**

Replace with:

```go
p.Total = p.Founders  // Founders are permanent, always count them
for _, cohort := range p.Cohorts {
    p.Total += cohort.Population
}
```

- [ ] **Step 3: Verify with a test scenario**

Start a city with 500 founders → check `city.population.total` includes 500. Let a heartbeat pass → founders should still be part of total.

- [ ] **Step 4: Commit**

```bash
git add internal/city/population.go
git commit -m "fix: include Founders in population total calculation"
```

---

## Chunk 2: HTML Layout & CSS Grid Restructuring

### Task 6: Restructure index.html for CSS Grid 1080p layout

**Files:**
- Modify: `web/index.html` (major rewrite of structure)

- [ ] **Step 1: Backup and plan the new structure**

The new layout has 3 main grid areas:
1. HUD (top, fixed 48px)
2. Game area (middle, 55vh) with 2 columns: canvas (1fr) + resources (300px)
3. Trade area (bottom, remaining 1fr) with 2 columns: my-products (1fr) + market (1fr)

- [ ] **Step 2: Replace the body HTML structure**

Current structure has `#login-screen`, `#hud`, `#game-container`, `#heartbeat-banner`, `#event-log`.

New structure should be:
```html
<body>
  <div id="login-screen"><!-- unchanged --></div>

  <div id="app-grid" style="display: none;">
    <div id="hud"><!-- unchanged --></div>

    <div id="game-area">
      <div id="game-container">
        <canvas id="phaser-canvas"></canvas>
      </div>
      <div id="resources-panel">
        <div class="panel-title">RESOURCES</div>
        <div id="resources-list"></div>
      </div>
    </div>

    <div id="trade-area">
      <div id="my-products-panel">
        <div class="panel-title">MY PRODUCTS</div>
        <div id="my-products-list"></div>
      </div>
      <div id="trade-market-panel">
        <div class="panel-title">TRADE MARKET</div>
        <div id="market-orders-list"></div>
      </div>
    </div>

    <div id="heartbeat-banner"><!-- unchanged --></div>
    <div id="event-log"><!-- unchanged --></div>
  </div>

  <!-- scripts -->
</body>
```

- [ ] **Step 3: Add CSS Grid rules to the `<style>` section**

Add before `</style>`:

```css
#app-grid {
  display: grid;
  grid-template-rows: 48px 55vh 1fr;
  height: 100vh;
  width: 100vw;
}

#game-area {
  display: grid;
  grid-template-columns: 1fr 300px;
  gap: 0;
}

#game-container {
  grid-column: 1 / 2;
  overflow: auto;
}

#resources-panel {
  grid-column: 2 / 3;
  background: #0f0f1a;
  border-left: 1px solid #333;
  overflow-y: auto;
  padding: 12px;
}

#trade-area {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
  border-top: 1px solid #333;
}

#my-products-panel,
#trade-market-panel {
  padding: 12px;
  overflow-y: auto;
  background: #0f0f1a;
}

#trade-market-panel {
  border-left: 1px solid #333;
}

.panel-title {
  color: #FFD700;
  font-size: 0.9rem;
  letter-spacing: 2px;
  margin-bottom: 12px;
  font-weight: bold;
}

/* Responsive: < 768px */
@media (max-width: 768px) {
  #app-grid {
    grid-template-rows: 48px 40vh auto auto auto;
  }

  #game-area {
    grid-template-columns: 1fr;
  }

  #resources-panel {
    grid-column: 1 / 2;
    border-left: none;
    border-top: 1px solid #333;
    max-height: 150px;
  }

  #trade-area {
    grid-template-columns: 1fr;
  }

  #trade-market-panel {
    border-left: none;
    border-top: 1px solid #333;
  }
}
```

- [ ] **Step 4: Test in browser**

Run `docker-compose up`, open `http://localhost:8080`.

Register and login → verify the grid layout appears with 4 panels arranged correctly.

- [ ] **Step 5: Commit**

```bash
git add web/index.html
git commit -m "feat: restructure HTML for CSS Grid 1080p layout with resources and trade panels"
```

---

## Chunk 3: Session Persistence (localStorage)

### Task 7: Add localStorage session persistence

**Files:**
- Modify: `web/src/main.js`

- [ ] **Step 1: Add session save on successful registration**

Find the `startGame()` function where registration succeeds (around line 50):

```js
const data = await resp.json();
gameState.playerID = data.player_id;
gameState.cityID = data.city_id;
gameState.city = data.city;
```

After these lines, add:

```js
// Save session to localStorage
const session = {
  playerID: data.player_id,
  cityID: data.city_id,
  playerName: playerName,
  cityName: cityName,
  timestamp: Date.now()
};
localStorage.setItem('cities_session', JSON.stringify(session));
```

- [ ] **Step 2: Add checkSavedSession function at the top of main.js**

Add before `startGame()`:

```js
function checkSavedSession() {
  const session = localStorage.getItem('cities_session');
  if (!session) return null;
  try {
    return JSON.parse(session);
  } catch (e) {
    return null;
  }
}
```

- [ ] **Step 3: Add resumeSession function**

```js
function resumeSession(session) {
  gameState.playerID = session.playerID;
  gameState.cityID = session.cityID;
  // Auto-connect without showing login
  showGame();
}
```

- [ ] **Step 4: Modify page load to check for saved session**

Add to the very beginning of `<script>` execution (after gameState declaration, before form handlers):

```js
document.addEventListener('DOMContentLoaded', () => {
  const savedSession = checkSavedSession();
  if (savedSession) {
    // Show "welcome back" screen instead of login
    const loginScreen = document.getElementById('login-screen');
    loginScreen.innerHTML = `
      <div class="pixel-title">CITIES</div>
      <div class="pixel-subtitle">Social Simulator — Multi-Player City Builder</div>
      <p style="color: #888; margin-bottom: 24px;">Welcome back!</p>
      <p style="color: #FFD700; font-size: 1.2rem; margin-bottom: 12px;">${savedSession.playerName}</p>
      <p style="color: #4A9EFF; margin-bottom: 32px;">${savedSession.cityName}</p>
      <button class="pixel-btn" onclick="resumeSession(checkSavedSession())">CONTINUAR</button>
      <button class="pixel-btn" style="margin-top: 12px; background: #666;" onclick="
        localStorage.removeItem('cities_session');
        location.reload();
      ">NUEVA CIUDAD</button>
    `;
  }
});
```

- [ ] **Step 5: Test**

1. Register a new city
2. Refresh the page → should show "Welcome back" screen
3. Click CONTINUAR → auto-connects
4. Click NUEVA CIUDAD → clears session and shows login form

- [ ] **Step 6: Commit**

```bash
git add web/src/main.js
git commit -m "feat: add localStorage session persistence with resume on page load"
```

---

## Chunk 4: Animated Counters & Population Arrows

### Task 8: Implement animated counter system

**Files:**
- Modify: `web/src/main.js`

- [ ] **Step 1: Add global tracking for previous values**

Near the top after gameState declaration:

```js
let prevHUD = {
  population: 0,
  treasury: 0,
  happiness: 0,
  round: 0
};

let hudAnimations = {}; // Track in-progress animations
```

- [ ] **Step 2: Add animateCounter helper function**

```js
function animateCounter(elementId, fromValue, toValue, duration = 600) {
  // Cancel any existing animation for this element
  if (hudAnimations[elementId]) {
    cancelAnimationFrame(hudAnimations[elementId].frameId);
  }

  const element = document.getElementById(elementId);
  const startTime = performance.now();
  const difference = toValue - fromValue;

  // Store original color
  const originalColor = element.style.color || getComputedStyle(element).color;

  const animate = (currentTime) => {
    const elapsed = currentTime - startTime;
    const progress = Math.min(elapsed / duration, 1);

    // Easing: ease-out-quad
    const easeProgress = 1 - (1 - progress) * (1 - progress);

    const current = Math.round(fromValue + difference * easeProgress);
    element.textContent = current.toLocaleString();

    // Flash yellow for first 300ms
    if (elapsed < 300) {
      element.style.color = '#FFD700';
    } else {
      element.style.color = originalColor;
    }

    if (progress < 1) {
      hudAnimations[elementId].frameId = requestAnimationFrame(animate);
    } else {
      delete hudAnimations[elementId];
      element.style.color = originalColor;
    }
  };

  hudAnimations[elementId] = { frameId: null };
  hudAnimations[elementId].frameId = requestAnimationFrame(animate);
}
```

- [ ] **Step 3: Modify updateHUD to use animation**

Find the current `updateHUD()` function. Replace numeric updates with animations:

Old:
```js
document.getElementById('hud-pop').textContent = (city.population?.total || 0).toLocaleString();
document.getElementById('hud-treasury').textContent = (city.treasury || 0).toLocaleString() + ' ¢';
document.getElementById('hud-happy').textContent = (city.happiness || 0).toFixed(1) + '%';
```

New:
```js
const newPop = city.population?.total || 0;
const newTreasury = city.treasury || 0;
const newHappy = city.happiness || 0;

if (newPop !== prevHUD.population) {
  animateCounter('hud-pop', prevHUD.population, newPop);
  prevHUD.population = newPop;
}

if (newTreasury !== prevHUD.treasury) {
  animateCounter('hud-treasury', prevHUD.treasury, newTreasury);
  prevHUD.treasury = newTreasury;
}

if (newHappy !== prevHUD.happiness) {
  animateCounter('hud-happy', prevHUD.happiness, newHappy, 600);
  prevHUD.happiness = newHappy;
}
```

- [ ] **Step 4: Test**

Trigger a heartbeat → watch numbers animate smoothly and flash yellow.

- [ ] **Step 5: Commit**

```bash
git add web/src/main.js
git commit -m "feat: add animated counter system with easing and flash effect"
```

---

### Task 9: Add population arrows with delta display

**Files:**
- Modify: `web/src/main.js`

- [ ] **Step 1: Add arrow rendering helper**

```js
function renderDelta(current, previous, isNegativeGood = false) {
  const delta = current - previous;
  if (delta === 0) return '—';

  const arrow = delta > 0 ? '↑' : '↓';
  const color = delta > 0 ? '#00FF88' : '#FF4444';
  const sign = delta > 0 ? '+' : '';

  return `<span style="color: ${color}; margin-left: 8px;">${arrow}${sign}${delta}</span>`;
}
```

- [ ] **Step 2: Extend HUD to show arrows**

Modify `updateHUD()` to add deltas after each number. After the animation code above:

```js
// Add arrows
const popDelta = renderDelta(newPop, prevCity?.population?.total || 0);
const treasuryDelta = renderDelta(newTreasury, prevCity?.treasury || 0);
const happyDelta = renderDelta(newHappy, prevCity?.happiness || 0);

document.getElementById('hud-pop').innerHTML = `${newPop.toLocaleString()} ${popDelta}`;
document.getElementById('hud-treasury').innerHTML = `${newTreasury.toLocaleString()} ¢ ${treasuryDelta}`;
document.getElementById('hud-happy').innerHTML = `${newHappy.toFixed(1)}% ${happyDelta}`;
```

- [ ] **Step 3: Track previous city state**

Add global at top:

```js
let prevCity = null;
```

Update it in the `city_update` handler (in `initWebSocket()`):

```js
.on('city_update', (update) => {
  if (update && update.city) {
    gameState.city = update.city;
    updateHUD(update.city);
    prevCity = update.city; // Save for next delta calc
    // ...rest of handler
  }
})
```

- [ ] **Step 4: Test**

Trigger heartbeat → see population change with arrows showing direction and magnitude.

Arrows persist until next heartbeat (because they're recalculated each update).

- [ ] **Step 5: Commit**

```bash
git add web/src/main.js
git commit -m "feat: add population arrows and delta display that persist per heartbeat"
```

---

## Chunk 5: Resources Panel & Trade UI

### Task 10: Build resources panel with full catalog

**Files:**
- Modify: `web/src/main.js` (add resource catalog and rendering)
- Modify: `web/index.html` (add resources HTML and styling)

- [ ] **Step 1: Add resource catalog to main.js**

```js
const RESOURCE_CATALOG = [
  // Tier 1
  { key: 'wood',           emoji: '🪵', name: 'Madera',        tier: 1 },
  { key: 'stone',          emoji: '🪨', name: 'Piedra',        tier: 1 },
  { key: 'iron',           emoji: '⛓️', name: 'Hierro',        tier: 1 },
  { key: 'copper',         emoji: '🥉', name: 'Cobre',         tier: 1 },
  { key: 'silicon',        emoji: '⏳', name: 'Silicio',       tier: 1 },
  { key: 'water',          emoji: '💧', name: 'Agua',          tier: 1 },
  { key: 'wheat',          emoji: '🌾', name: 'Trigo',         tier: 1 },
  { key: 'oil',            emoji: '🛢️', name: 'Petróleo',      tier: 1 },
  { key: 'wool',           emoji: '🐑', name: 'Lana',          tier: 1 },
  { key: 'rubber',         emoji: '🌳', name: 'Caucho',        tier: 1 },
  // Tier 2 — Goods
  { key: 'steel_beams',    emoji: '🏗️', name: 'Vigas',         tier: 2 },
  { key: 'bricks',         emoji: '🧱', name: 'Ladrillos',     tier: 2 },
  { key: 'tools',          emoji: '🛠️', name: 'Herramientas',  tier: 2 },
  { key: 'wiring',         emoji: '🔌', name: 'Cableado',      tier: 2 },
  { key: 'bread',          emoji: '🍞', name: 'Pan',           tier: 2 },
  { key: 'clothing',       emoji: '👕', name: 'Ropa',          tier: 2 },
  { key: 'gasoline',       emoji: '⛽', name: 'Gasolina',      tier: 2 },
  { key: 'furniture',      emoji: '🪑', name: 'Muebles',       tier: 2 },
  { key: 'tires',          emoji: '🛞', name: 'Neumáticos',    tier: 2 },
  { key: 'glass',          emoji: '🍷', name: 'Vidrio',        tier: 2 },
  // Tier 2 — Services
  { key: 'energy',         emoji: '⚡', name: 'Energía',       tier: 2 },
  { key: 'waste',          emoji: '🗑️', name: 'Basura',        tier: 2 },
  { key: 'security',       emoji: '👮', name: 'Seguridad',     tier: 2 },
  { key: 'education',      emoji: '🎒', name: 'Educación',     tier: 2 },
  { key: 'health',         emoji: '🚑', name: 'Salud',         tier: 2 },
  { key: 'transport',      emoji: '🚌', name: 'Transporte',    tier: 2 },
  { key: 'entertainment',  emoji: '📻', name: 'Radio',         tier: 2 },
  { key: 'logistics',      emoji: '📦', name: 'Logística',     tier: 2 },
  { key: 'maintenance',    emoji: '🧹', name: 'Mantenimiento', tier: 2 },
  { key: 'water_treatment',emoji: '🚽', name: 'Agua Trat.',    tier: 2 },
];

// Track previous resource values for arrows
let prevResources = {};
```

- [ ] **Step 2: Add updateResourcesPanel function**

```js
function updateResourcesPanel(resources) {
  const panel = document.getElementById('resources-list');
  const tier1 = RESOURCE_CATALOG.filter(r => r.tier === 1);
  const tier2 = RESOURCE_CATALOG.filter(r => r.tier === 2);

  let html = '';

  // Tier 1
  html += '<div style="color: #888; font-size: 0.75rem; margin-bottom: 8px; letter-spacing: 1px;">TIER 1</div>';
  tier1.forEach(resource => {
    const qty = resources[resource.key] || 0;
    const prev = prevResources[resource.key] || 0;
    const delta = renderDelta(qty, prev);
    const opacity = qty === 0 ? 'opacity: 0.4;' : '';

    html += `
      <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.8rem; margin-bottom: 6px; ${opacity}">
        <span>${resource.emoji} ${resource.name}</span>
        <span style="color: #FFD700;">${qty} ${delta}</span>
      </div>
    `;
  });

  // Tier 2
  html += '<div style="color: #888; font-size: 0.75rem; margin-top: 12px; margin-bottom: 8px; letter-spacing: 1px;">TIER 2</div>';
  tier2.forEach(resource => {
    const qty = resources[resource.key] || 0;
    const prev = prevResources[resource.key] || 0;
    const delta = renderDelta(qty, prev);
    const opacity = qty === 0 ? 'opacity: 0.4;' : '';

    html += `
      <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.8rem; margin-bottom: 6px; ${opacity}">
        <span>${resource.emoji} ${resource.name}</span>
        <span style="color: #FFD700;">${qty} ${delta}</span>
      </div>
    `;
  });

  panel.innerHTML = html;

  // Update previous values for next time
  RESOURCE_CATALOG.forEach(r => {
    prevResources[r.key] = resources[r.key] || 0;
  });
}
```

- [ ] **Step 3: Call updateResourcesPanel in updateHUD**

At the end of `updateHUD()`:

```js
const resources = city.resources || {};
updateResourcesPanel(resources);
```

- [ ] **Step 4: Add CSS styling for resources panel**

In `<style>`:

```css
#resources-list {
  display: flex;
  flex-direction: column;
}

#resources-list > div {
  word-break: break-word;
}
```

- [ ] **Step 5: Test**

Trigger heartbeat → resources panel should show all resources with quantities and arrows for changes.

- [ ] **Step 6: Commit**

```bash
git add web/src/main.js web/index.html
git commit -m "feat: add full resources panel with Tier 1 & 2 catalog and delta arrows"
```

---

### Task 11: Add trade market UI structure and polling

**Files:**
- Modify: `web/src/main.js`
- Modify: `web/index.html` (add market HTML table structure)

- [ ] **Step 1: Add market polling function**

```js
let lastMarketPoll = 0;
const MARKET_POLL_INTERVAL = 30000; // 30 seconds

async function updateMarketOrders() {
  try {
    const response = await fetch('/api/trade/orders');
    if (!response.ok) return;

    const orders = await response.json();
    if (!Array.isArray(orders)) return;

    renderMarketOrders(orders);
  } catch (err) {
    console.error('[Market] Error fetching orders:', err);
  }
}

function renderMarketOrders(orders) {
  const panel = document.getElementById('market-orders-list');

  if (!orders || orders.length === 0) {
    panel.innerHTML = '<div style="color: #666; text-align: center; padding: 20px;">No orders available</div>';
    return;
  }

  let html = '<div style="font-size: 0.75rem; color: #888; display: grid; grid-template-columns: 1fr 1fr 0.5fr 0.5fr 0.8fr; gap: 8px; margin-bottom: 12px; border-bottom: 1px solid #333; padding-bottom: 8px;"><div>City</div><div>Product</div><div>Qty</div><div>Price</div><div>Action</div></div>';

  orders.forEach(order => {
    const icon = RESOURCE_CATALOG.find(r => r.key === order.Product)?.emoji || '?';
    const name = RESOURCE_CATALOG.find(r => r.key === order.Product)?.name || order.Product;

    html += `
      <div style="font-size: 0.75rem; display: grid; grid-template-columns: 1fr 1fr 0.5fr 0.5fr 0.8fr; gap: 8px; align-items: center; padding: 6px 0; border-bottom: 1px solid #222;">
        <div style="color: #FFD700;">${order.CityID.substring(0, 12)}</div>
        <div>${icon} ${name}</div>
        <div>${order.Quantity}</div>
        <div style="color: #00FF88;">${order.Price}¢</div>
        <button class="pixel-btn" style="padding: 4px 8px; font-size: 0.7rem;" onclick="buyOrder('${order.ID}', '${order.Product}', ${order.Quantity}, ${order.Price})">BUY</button>
      </div>
    `;
  });

  panel.innerHTML = html;
}

function buyOrder(orderId, product, quantity, price) {
  if (!gameState.cityID) return;

  const buyQty = prompt(`Buy how much ${product}? (available: ${quantity})`);
  if (!buyQty || isNaN(buyQty) || buyQty <= 0) return;

  const body = {
    city_id: gameState.cityID,
    product: product,
    side: 'buy',
    quantity: parseInt(buyQty),
    price: price
  };

  fetch('/api/trade/orders', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  })
    .then(r => r.json())
    .then(order => {
      addLog(`Buy order placed: ${buyQty}× ${product} @ ${price}¢`, 'good');
      updateMarketOrders();
    })
    .catch(err => {
      addLog(`Error placing order: ${err.message}`, 'bad');
    });
}
```

- [ ] **Step 2: Call updateMarketOrders on heartbeat and periodically**

In `initWebSocket()`, add to the `city_update` handler:

```js
.on('city_update', (update) => {
  if (update && update.city) {
    gameState.city = update.city;
    updateHUD(update.city);
    updateMarketOrders(); // Refresh market on city update
    prevCity = update.city;
    // ...rest
  }
})
```

And set up periodic polling when game starts:

```js
function showGame() {
  // ...existing code

  // Polling for market orders every 30s
  setInterval(() => {
    if (gameState.cityID) {
      updateMarketOrders();
    }
  }, MARKET_POLL_INTERVAL);
}
```

- [ ] **Step 3: Add HTML table structure to index.html**

In `#trade-market-panel`:

```html
<div id="market-orders-list">
  <div style="color: #666; text-align: center; padding: 20px;">Loading market...</div>
</div>
```

- [ ] **Step 4: Test**

Register → trigger heartbeat → market orders should appear if backend has any.

Try placing a buy order via the button → should appear in console logs.

- [ ] **Step 5: Commit**

```bash
git add web/src/main.js web/index.html
git commit -m "feat: add trade market UI with polling and buy order placement"
```

---

### Task 12: Add "My Products" sell interface

**Files:**
- Modify: `web/src/main.js`

- [ ] **Step 1: Add base price mapping**

```js
const PRODUCT_BASE_PRICES = {
  // Tier 1 raw materials
  'wood': 5,
  'stone': 5,
  'iron': 8,
  'copper': 7,
  'silicon': 10,
  'water': 2,
  'wheat': 4,
  'oil': 12,
  'wool': 6,
  'rubber': 8,
  // Tier 2 goods
  'bread': 15,
  'steel_beams': 20,
  'bricks': 10,
  'tools': 25,
  'wiring': 18,
  'clothing': 20,
  'gasoline': 14,
  'furniture': 22,
  'tires': 16,
  'glass': 12,
  // Tier 2 services
  'energy': 30,
  'waste': 5,
  'security': 25,
  'education': 35,
  'health': 40,
  'transport': 28,
  'entertainment': 20,
  'logistics': 22,
  'maintenance': 18,
  'water_treatment': 15
};

function calculateSuggestedPrice(product, stock) {
  const base = PRODUCT_BASE_PRICES[product] || 10;
  // High stock → lower price (0.5x), low stock → higher price (3x)
  const multiplier = Math.max(0.5, Math.min(3.0, 200 / Math.max(stock, 1)));
  return Math.round(base * multiplier);
}
```

- [ ] **Step 2: Add renderMyProducts function**

```js
function renderMyProducts(resources) {
  const panel = document.getElementById('my-products-list');

  // Only show resources where we have stock > 0
  const productsToSell = RESOURCE_CATALOG.filter(r => (resources[r.key] || 0) > 0);

  if (productsToSell.length === 0) {
    panel.innerHTML = '<div style="color: #666; text-align: center; padding: 20px;">No products to sell</div>';
    return;
  }

  let html = '<div style="font-size: 0.75rem; color: #888; display: grid; grid-template-columns: 1.5fr 0.5fr 1fr 0.8fr; gap: 8px; margin-bottom: 12px; border-bottom: 1px solid #333; padding-bottom: 8px;"><div>Product</div><div>Stock</div><div>Price</div><div>Action</div></div>';

  productsToSell.forEach(product => {
    const stock = resources[product.key] || 0;
    const suggestedPrice = calculateSuggestedPrice(product.key, stock);

    html += `
      <div style="font-size: 0.75rem; display: grid; grid-template-columns: 1.5fr 0.5fr 1fr 0.8fr; gap: 8px; align-items: center; padding: 6px 0; border-bottom: 1px solid #222;">
        <div>${product.emoji} ${product.name}</div>
        <div style="color: #FFD700;">${stock}</div>
        <div style="color: #4A9EFF;">${suggestedPrice}¢</div>
        <button class="pixel-btn" style="padding: 4px 8px; font-size: 0.7rem;" onclick="sellProduct('${product.key}', ${stock}, ${suggestedPrice})">SELL</button>
      </div>
    `;
  });

  panel.innerHTML = html;
}

function sellProduct(product, maxStock, suggestedPrice) {
  const qty = prompt(`Sell how much ${product}? (available: ${maxStock})`);
  if (!qty || isNaN(qty) || qty <= 0 || qty > maxStock) return;

  const price = prompt(`Price per unit? (suggested: ${suggestedPrice}¢)`, suggestedPrice);
  if (!price || isNaN(price) || price <= 0) return;

  const body = {
    city_id: gameState.cityID,
    product: product,
    side: 'sell',
    quantity: parseInt(qty),
    price: parseInt(price)
  };

  fetch('/api/trade/orders', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  })
    .then(r => r.json())
    .then(order => {
      addLog(`Sell order placed: ${qty}× ${product} @ ${price}¢`, 'good');
      renderMyProducts(gameState.city?.resources || {});
      updateMarketOrders(); // Refresh market to show our new order
    })
    .catch(err => {
      addLog(`Error placing sell order: ${err.message}`, 'bad');
    });
}
```

- [ ] **Step 3: Call renderMyProducts in updateHUD**

At the end of `updateHUD()`:

```js
renderMyProducts(resources);
```

- [ ] **Step 4: Add HTML structure to index.html**

In `#my-products-panel`:

```html
<div id="my-products-list">
  <div style="color: #666; text-align: center; padding: 20px;">Loading products...</div>
</div>
```

- [ ] **Step 5: Test**

Register → buy some resources via trade market or wait for production → should appear in "My Products" panel.

Click SELL → enter quantity and price → order should appear in market.

- [ ] **Step 6: Commit**

```bash
git add web/src/main.js web/index.html
git commit -m "feat: add my products panel with dynamic pricing and sell orders"
```

---

## Chunk 6: Canvas Resize & Final Integration

### Task 13: Resize Phaser canvas to full viewport

**Files:**
- Modify: `web/src/main.js`
- Modify: `web/src/scenes/GameScene.js`

- [ ] **Step 1: Update initPhaser to use calculated canvas size**

Replace the hardcoded config in `initPhaser()`:

Old:
```js
const config = {
  type: Phaser.CANVAS,
  width: Math.min(window.innerWidth, 800),
  height: 400,
  ...
};
```

New:
```js
const config = {
  type: Phaser.CANVAS,
  width: window.innerWidth - 300,  // Leave room for resources panel
  height: Math.floor(window.innerHeight * 0.55),  // 55vh
  ...
};
```

But also handle responsive resizing:

```js
function initPhaser() {
  function getCanvasSize() {
    const isMobile = window.innerWidth < 768;
    const width = isMobile ? window.innerWidth : window.innerWidth - 300;
    const height = isMobile ? Math.floor(window.innerHeight * 0.4) : Math.floor(window.innerHeight * 0.55);
    return { width: Math.max(width, 480), height: Math.max(height, 300) };
  }

  const size = getCanvasSize();

  const config = {
    type: Phaser.CANVAS,
    width: size.width,
    height: size.height,
    canvas: document.getElementById('phaser-canvas'),
    backgroundColor: '#0a0a0f',
    pixelArt: true,
    antialias: false,
    scene: [GameScene],
  };

  window.phaserGame = new Phaser.Game(config);

  // Handle window resize
  window.addEventListener('resize', () => {
    const newSize = getCanvasSize();
    window.phaserGame.scale.resize(newSize.width, newSize.height);
  });
}
```

- [ ] **Step 2: Verify GameScene uses dynamic W/H**

Open `web/src/scenes/GameScene.js` and verify that it uses `this.scale.width` and `this.scale.height` for all dimensions (it should already).

No changes needed in GameScene itself.

- [ ] **Step 3: Test responsive behavior**

Run `docker-compose up`.

1. Resize browser to wide (>768px) → canvas should be narrower, resources panel visible on right
2. Resize browser to mobile (<768px) → full-width canvas, resources panel below (due to CSS media query)

- [ ] **Step 4: Commit**

```bash
git add web/src/main.js
git commit -m "feat: resize Phaser canvas to fill viewport with responsive sizing"
```

---

### Task 14: Final integration testing and small fixes

**Files:**
- Various

- [ ] **Step 1: Full end-to-end test**

```bash
docker-compose up
```

1. Open `http://localhost:8080`
2. Register new player and city
3. Check that localStorage saved session
4. Refresh page → "Welcome back" screen appears
5. Click CONTINUAR → game loads without showing login form
6. View HUD with animated counters
7. Wait for heartbeat → see population arrows
8. View resources panel with all 30 items
9. View my-products panel
10. View trade market panel (empty if no other orders)
11. Try placing a sell order
12. Resize browser to mobile width → layout stacks correctly

- [ ] **Step 2: Test market order placement**

Open two browser tabs. In tab 1: register City A. In tab 2: register City B.

In tab 1: Place a sell order for bread (10 units @ 20¢)
In tab 2: Refresh → should see City A's order in market
In tab 2: Click BUY → enter quantity → order placed as buy
In both tabs: Wait for heartbeat → trade should settle

- [ ] **Step 3: Fix any CSS alignment issues**

If panels don't align perfectly, adjust widths and padding in the CSS grid.

- [ ] **Step 4: Test mobile responsiveness**

Open DevTools → toggle device toolbar → test on iPhone/iPad sizes.

Panels should stack vertically, canvas should resize smoothly.

- [ ] **Step 5: Final commit with summary**

```bash
git add -A
git commit -m "feat: complete Cities UI overhaul with full 1080p layout, trade market, and responsive design

- CSS Grid 1080p layout with resources and trade panels
- localStorage session persistence with resume on page load
- Animated counters with easing and yellow flash effect
- Population arrows and delta display per heartbeat
- Full Tier 1 & 2 resource catalog (30 items)
- Trade market order book with buy orders
- My Products panel with dynamic pricing and sell orders
- Responsive layout for mobile (<768px)
- Phaser canvas resizes to fill viewport
- Bug fixes: heartbeat_id reference and founders count"
```

- [ ] **Step 6: Verify no console errors**

Open browser DevTools console → no red errors should appear during normal gameplay.

---

## Summary

**Total commits:** 14 (one per step in tasks)

**Key architectural decisions:**
1. Backend only adds 2 REST endpoints; no WebSocket changes needed
2. Frontend uses localStorage for session, not cookies (simpler for SPA)
3. Market updates via polling (30s interval) + on city_update events
4. CSS Grid handles responsive layout with media query for mobile
5. Animated counters use requestAnimationFrame for smooth 60fps rendering
6. Resources panel uses opacity 0.4 for zero quantities as visual cue

**Testing approach:**
- Manual browser testing for UI/UX
- Curl testing for backend endpoints
- Docker Compose for full integration
