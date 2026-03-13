/**
 * Cities — Social Simulator
 * Main entry point for the Phaser.js 8-bit web client.
 */

let currentHeartbeat = null;
let selectedProposalID = null;
let prevCity = null;
let prevHUD = {
  population: 0,
  treasury: 0,
  happiness: 0
};
let hudAnimations = {};

const RESOURCE_CATALOG = [
  // Raw Materials (generados por el backend)
  { key: 'materials',      emoji: '📦', name: 'Materiales',    tier: 1 },
  { key: 'metal',          emoji: '⛓️', name: 'Metal',         tier: 1 },
  { key: 'food',           emoji: '🌾', name: 'Alimentos',     tier: 1 },
  { key: 'medicines',      emoji: '💊', name: 'Medicinas',     tier: 1 },
];

let prevResources = {};

window.gameState = {
  playerID: null,
  cityID: null,
  city: null,
};
const gameState = window.gameState;

// ─── Session Persistence ─────────────────────────────────────────────────────

function checkSavedSession() {
  const session = localStorage.getItem('cities_session');
  if (!session) return null;
  try {
    return JSON.parse(session);
  } catch (e) {
    return null;
  }
}

function resumeSession(session) {
  gameState.playerID = session.playerID;
  gameState.cityID = session.cityID;
  showGame();
}

// ─── Login / Registration ────────────────────────────────────────────────────

async function startGame() {
  const playerName = document.getElementById('player-name').value.trim();
  const cityName = document.getElementById('city-name').value.trim();
  const existingPlayerID = document.getElementById('existing-player-id').value.trim();
  const existingCityID = document.getElementById('existing-city-id').value.trim();
  const errorEl = document.getElementById('error-msg');

  console.log('[Game] Starting registration...', { playerName, cityName });

  if (!playerName) {
    errorEl.textContent = 'Please enter your name';
    return;
  }
  if (!cityName && !existingPlayerID) {
    errorEl.textContent = 'Please enter a city name';
    return;
  }

  errorEl.textContent = '';

  try {
    if (existingPlayerID && existingCityID) {
      gameState.playerID = existingPlayerID;
      gameState.cityID = existingCityID;
    } else {
      const resp = await fetch('/api/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ player_name: playerName, city_name: cityName }),
      });
      if (!resp.ok) {
        const txt = await resp.text();
        throw new Error(txt || `Server error: ${resp.status}`);
      }
      const data = await resp.json();
      console.log('[Game] Registered successfully:', data);
      gameState.playerID = data.player_id;
      gameState.cityID = data.city_id;
      gameState.city = data.city;

      // Save session to localStorage
      const session = {
        playerID: data.player_id,
        cityID: data.city_id,
        playerName: playerName,
        cityName: cityName,
        timestamp: Date.now()
      };
      localStorage.setItem('cities_session', JSON.stringify(session));
    }

    console.log('[Game] Transitioning to game view...');
    showGame();
  } catch (err) {
    console.error('[Game] Initialization failed:', err);
    errorEl.textContent = `Failed: ${err.message}`;
  }
}

function showGame() {
  document.getElementById('login-screen').style.display = 'none';
  document.getElementById('app-grid').style.display = 'block';
  document.getElementById('hud').style.display = 'block';

  initPhaser();
  initWebSocket();

  // Polling for market orders
  setInterval(() => {
    if (gameState.cityID) {
      updateMarketOrders();
    }
  }, MARKET_POLL_INTERVAL);

  if (gameState.city) {
    updateHUD(gameState.city);
  }
}

// ─── Phaser Game ─────────────────────────────────────────────────────────────

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

// ─── WebSocket ────────────────────────────────────────────────────────────────

function initWebSocket() {
  const serverURL = window.location.origin;
  window.citiesWS = new CitiesWS(serverURL, gameState.playerID, gameState.cityID);

  window.citiesWS
    .on('connected', () => {
      addLog('Connected to coordinator', 'good');
    })
    .on('disconnected', () => {
      addLog('Disconnected — reconnecting...', 'bad');
    })
    .on('heartbeat', (hb) => {
      showHeartbeat(hb);
    })
    .on('city_update', (update) => {
      if (update && update.city) {
        gameState.city = update.city;
        console.log('[CITY_UPDATE] Resources received:', update.city.resources);
        console.log('[CITY_UPDATE] Full city:', update.city);
        updateHUD(update.city);
        updateCityNeeds(update.city);
        updateMarketOrders();
        renderMyProducts(update.city.resources || {});
        addLog(`City updated — pop: ${update.city.population?.total || 0}`, 'good');
        if (update.events) {
          update.events.forEach(ev => {
            addLog(`${ev.type}: ${ev.count} ${ev.group} — ${ev.reason}`);
          });
        }
      }
    })
    .on('world_event', (event) => {
      if (event) {
        addLog(`[WORLD] ${event.description}`, 'important');
      }
    });

  window.citiesWS.connect();

  if (gameState.city) {
    updateHUD(gameState.city);
  }
}

// ─── Heartbeat UI ─────────────────────────────────────────────────────────────

function showHeartbeat(hb) {
  currentHeartbeat = hb;
  selectedProposalID = null;

  const banner = document.getElementById('heartbeat-banner');
  const container = document.getElementById('proposals-container');
  const submitBtn = document.getElementById('submit-decision');

  container.innerHTML = '';

  (hb.proposals || []).forEach((proposal) => {
    const card = document.createElement('div');
    card.className = 'proposal-card';
    card.dataset.id = proposal.id;

    const icon = getInitiativeIcon(proposal.type);
    const eff = proposal.effects || {};
    const popSign = eff.population_delta >= 0 ? '+' : '';
    const happySign = eff.happiness_delta >= 0 ? '+' : '';
    const moneySign = eff.treasury_delta >= 0 ? '+' : '';

    card.innerHTML = `
      <h4>${icon} ${proposal.title}</h4>
      <p>${proposal.description}</p>
      <p class="cost">Cost: ${(proposal.cost || 0).toLocaleString()} ¢</p>
      <div class="effects">
        Pop ${popSign}${eff.population_delta || 0} |
        Happy ${happySign}${(eff.happiness_delta || 0).toFixed(1)} |
        Treasury ${moneySign}${(eff.treasury_delta || 0).toLocaleString()}
      </div>
    `;

    card.addEventListener('click', () => selectProposal(proposal.id, card));
    container.appendChild(card);
  });

  banner.style.display = 'block';
  submitBtn.style.display = 'none';
  addLog(`Heartbeat! Round ${hb.round || 0} — ${(hb.proposals || []).length} proposals`, 'important');
}

function selectProposal(proposalID, cardEl) {
  selectedProposalID = proposalID;
  document.querySelectorAll('.proposal-card').forEach(c => c.classList.remove('selected'));
  cardEl.classList.add('selected');
  const submitBtn = document.getElementById('submit-decision');
  submitBtn.style.display = 'block';
  submitBtn.onclick = () => {
    const payload = {
      heartbeat_id: currentHeartbeat.id,
      initiative_id: proposalID
    };
    window.citiesWS.send('decision', payload);
    document.getElementById('heartbeat-banner').style.display = 'none';
    addLog('Decision submitted to council.', 'good');
  };
}

function submitDecision() {
  if (!selectedProposalID || !currentHeartbeat) return;
  if (!window.citiesWS || !window.citiesWS.connected) {
    addLog('Not connected to server', 'bad');
    return;
  }

  window.citiesWS.submitDecision(currentHeartbeat.id, selectedProposalID);

  const chosen = (currentHeartbeat.proposals || []).find(p => p.id === selectedProposalID);
  addLog(`Decision submitted: ${chosen ? chosen.title : selectedProposalID}`, 'good');

  document.getElementById('heartbeat-banner').style.display = 'none';
  currentHeartbeat = null;
  selectedProposalID = null;
}

// ─── Dev: Manual Heartbeat Trigger ────────────────────────────────────────────

async function triggerHeartbeat() {
  try {
    const response = await fetch('/api/debug/heartbeat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    });

    if (response.ok) {
      addLog('Heartbeat triggered manually', 'good');
    } else {
      addLog(`Trigger failed: ${response.status}`, 'bad');
    }
  } catch (err) {
    addLog(`Error triggering heartbeat: ${err.message}`, 'bad');
  }
}

// ─── Event Log ───────────────────────────────────────────────────────────────

function animateCounter(elementId, fromValue, toValue, duration = 1200) {
  if (hudAnimations[elementId]) {
    cancelAnimationFrame(hudAnimations[elementId].frameId);
  }

  const element = document.getElementById(elementId);
  const startTime = performance.now();
  const difference = toValue - fromValue;
  const originalColor = element.style.color || getComputedStyle(element).color;

  const animate = (currentTime) => {
    const elapsed = currentTime - startTime;
    const progress = Math.min(elapsed / duration, 1);
    const easeProgress = 1 - (1 - progress) * (1 - progress); // ease-out-quad

    const current = Math.round(fromValue + difference * easeProgress);
    element.textContent = current.toLocaleString();

    // Highlight color during animation
    if (elapsed < 600) {
      element.style.color = difference > 0 ? '#00FF88' : '#FF4444';
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

function renderDelta(current, previous) {
  const delta = current - previous;
  if (delta === 0) return '—';
  const arrow = delta > 0 ? '↑' : '↓';
  const color = delta > 0 ? '#00FF88' : '#FF4444';
  const sign = delta > 0 ? '+' : '';
  return `<span style="color: ${color}; margin-left: 8px;">${arrow}${sign}${delta}</span>`;
}

function updateResourcesPanel(resources) {
  console.log('[RESOURCES] Updating panel with:', resources);
  const panel = document.getElementById('resources-list');

  let html = '';

  // Show all resources
  RESOURCE_CATALOG.forEach(resource => {
    const qty = resources[resource.key] || 0;
    const prev = prevResources[resource.key] || 0;
    const delta = renderDelta(qty, prev);
    const isChanged = qty !== prev;

    const highlight = isChanged ? 'background: #2a2a3a; padding: 2px 4px; border-radius: 2px; font-weight: bold;' : '';
    const textColor = isChanged && qty > prev ? '#00FF88' : isChanged && qty < prev ? '#FF4444' : '#FFD700';

    html += `
      <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.95rem; margin-bottom: 10px;">
        <span style="flex: 1;">${resource.emoji} ${resource.name}</span>
        <span style="color: ${textColor}; ${highlight}">${qty}</span>
        <span style="color: #666; margin-left: 6px;">${delta}</span>
      </div>
    `;
  });

  panel.innerHTML = html;

  // Update previous values
  RESOURCE_CATALOG.forEach(r => {
    prevResources[r.key] = resources[r.key] || 0;
  });
}

// ─── City Needs Panel ─────────────────────────────────────────────────────────

function updateCityNeeds(city) {
  if (!city) {
    document.getElementById('needs-list').innerHTML = '<div style="color: #666; padding: 20px;">No city data</div>';
    return;
  }

  const pop = city.population?.total || 0;
  const happy = city.happiness || 50; // default 50%
  const unemploy = city.stats?.unemployment_rate || 0;
  const health = city.stats?.health_level || 50; // default 50%
  const education = city.stats?.education_level || 50; // default 50%
  const innovation = city.stats?.innovation_index || 0;
  const buildings = city.buildings?.length || 0;

  // RSI (Resident Satisfaction Index) = Happiness
  const rsi = happy.toFixed(0);
  const rsiColor = happy >= 70 ? '#00FF88' : happy >= 40 ? '#FFD700' : '#FF4444';

  // Housing need (1 house per 50 pop ideally)
  const housesNeeded = Math.ceil(pop / 50);
  const housesBuilt = buildings;
  const housingOK = housesBuilt >= housesNeeded ? '✓' : '✗';

  // Job need (1 job per 3 workers)
  const workers = city.population?.workers || 0;
  const jobsNeeded = Math.ceil(workers / 3);
  const jobsColor = unemploy > 20 ? '#FF4444' : unemploy > 10 ? '#FFD700' : '#00FF88';

  // Health indicator
  const healthColor = health >= 60 ? '#00FF88' : health >= 30 ? '#FFD700' : '#FF4444';

  // Education indicator
  const educColor = education >= 60 ? '#00FF88' : education >= 30 ? '#FFD700' : '#FF4444';

  const html = `
    <div style="font-size: 0.8rem; color: #888; line-height: 1.8;">
      <div style="margin-bottom: 12px; padding-bottom: 8px; border-bottom: 1px solid #333;">
        <div style="font-weight: bold; color: ${rsiColor}">RSI: ${rsi}%</div>
        <div style="font-size: 0.7rem; color: #666;">Resident Satisfaction</div>
      </div>

      <div style="margin-bottom: 10px;">
        <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
          <span>Employment:</span>
          <span style="color: ${jobsColor};">${unemploy.toFixed(0)}%</span>
        </div>
        <div style="font-size: 0.7rem; color: #666;">Need ${jobsNeeded} jobs</div>
      </div>

      <div style="margin-bottom: 10px;">
        <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
          <span>Houses:</span>
          <span>${housesBuilt}/${housesNeeded} ${housingOK}</span>
        </div>
        <div style="font-size: 0.7rem; color: #666;">Built / Needed</div>
      </div>

      <div style="margin-bottom: 10px;">
        <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
          <span>Health:</span>
          <span style="color: ${healthColor};">${health}%</span>
        </div>
        <div style="font-size: 0.7rem; color: #666;">City health level</div>
      </div>

      <div style="margin-bottom: 10px;">
        <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
          <span>Education:</span>
          <span style="color: ${educColor};">${education}%</span>
        </div>
        <div style="font-size: 0.7rem; color: #666;">Population education</div>
      </div>

      <div style="margin-bottom: 10px;">
        <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
          <span>Innovation:</span>
          <span>${innovation}</span>
        </div>
        <div style="font-size: 0.7rem; color: #666;">Tech index</div>
      </div>

      <div style="margin-top: 12px; padding-top: 8px; border-top: 1px solid #333; font-size: 0.7rem; color: #666; line-height: 1.6;">
        <div>🔴 Critical &lt;30%</div>
        <div>🟡 Warning 30-60%</div>
        <div>🟢 Good &gt;60%</div>
      </div>
    </div>
  `;

  document.getElementById('needs-list').innerHTML = html;
}

function updateHUD(city) {
  if (!city) return;
  console.log('[HUD] Updating with city data:', city.name, city.resources);

  const newPop = city.population?.total || 0;
  const newTreasury = city.treasury || 0;
  const newHappy = city.happiness || 0;

  // Animate population
  if (newPop !== prevHUD.population) {
    animateCounter('hud-pop', prevHUD.population, newPop);
    prevHUD.population = newPop;
  }
  const popDelta = renderDelta(newPop, prevCity?.population?.total || 0);
  document.getElementById('hud-pop').innerHTML = `${newPop.toLocaleString()} ${popDelta}`;

  // Animate treasury
  if (newTreasury !== prevHUD.treasury) {
    animateCounter('hud-treasury', prevHUD.treasury, newTreasury);
    prevHUD.treasury = newTreasury;
  }
  const treasuryDelta = renderDelta(newTreasury, prevCity?.treasury || 0);
  document.getElementById('hud-treasury').innerHTML = `${newTreasury.toLocaleString()} ¢ ${treasuryDelta}`;

  // Animate happiness
  if (newHappy !== prevHUD.happiness) {
    animateCounter('hud-happy', prevHUD.happiness, newHappy);
    prevHUD.happiness = newHappy;
  }
  const happyDelta = renderDelta(newHappy, prevCity?.happiness || 0);
  document.getElementById('hud-happy').innerHTML = `${newHappy.toFixed(1)}% ${happyDelta}`;

  // Rest of updateHUD (existing code for tax, round, etc.)
  document.getElementById('hud-city').textContent = city.name || '—';
  document.getElementById('hud-tax').textContent = (city.tax_rate || 0).toFixed(1) + '%';
  document.getElementById('hud-round').textContent = city.round || '0';

  // Resources (existing)
  const res = city.resources || {};
  const resContainer = document.getElementById('hud-resources');
  resContainer.innerHTML = `
    <span class="resource-badge res-food">FOOD: ${res.food || 0}</span>
    <span class="resource-badge res-metal">METAL: ${res.metal || 0}</span>
    <span class="resource-badge res-materials">MATS: ${res.materials || 0}</span>
    ${res.medicines ? `<span class="resource-badge res-medicines">MEDS: ${res.medicines}</span>` : ''}
  `;

  // Update prevCity for next delta calculation
  prevCity = city;

  // Update resources panel
  updateResourcesPanel(res);

  // Update city needs panel
  updateCityNeeds(city);
}

// ─── Task 11: Trade Market Polling & Order Book ──────────────────────────────

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
    .then(r => {
      if (!r.ok) {
        return r.json().then(e => Promise.reject(new Error(e.error || `HTTP ${r.status}`)));
      }
      return r.json();
    })
    .then(order => {
      addLog(`Buy order placed: ${buyQty}× ${product} @ ${price}¢`, 'good');
      updateMarketOrders();
    })
    .catch(err => {
      addLog(`Error placing order: ${err.message}`, 'bad');
    });
}

// ─── Task 12: My Products Panel ───────────────────────────────────────────────

const PRODUCT_BASE_PRICES = {
  'wood': 5, 'stone': 5, 'iron': 8, 'copper': 7, 'silicon': 10,
  'water': 2, 'wheat': 4, 'oil': 12, 'wool': 6, 'rubber': 8,
  'steel_beams': 20, 'bricks': 10, 'tools': 25, 'wiring': 18,
  'bread': 15, 'clothing': 20, 'gasoline': 14, 'furniture': 22,
  'tires': 16, 'glass': 12,
  'energy': 30, 'waste': 5, 'security': 25, 'education': 35,
  'health': 40, 'transport': 28, 'entertainment': 20,
  'logistics': 22, 'maintenance': 18, 'water_treatment': 15
};

function calculateSuggestedPrice(product, stock) {
  const base = PRODUCT_BASE_PRICES[product] || 10;
  const multiplier = Math.max(0.5, Math.min(3.0, 200 / Math.max(stock, 1)));
  return Math.round(base * multiplier);
}

function renderMyProducts(resources) {
  const panel = document.getElementById('my-products-list');

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
    .then(r => {
      if (!r.ok) {
        return r.json().then(e => Promise.reject(new Error(e.error || `HTTP ${r.status}`)));
      }
      return r.json();
    })
    .then(order => {
      addLog(`Sell order placed: ${qty}× ${product} @ ${price}¢`, 'good');
      renderMyProducts(gameState.city?.resources || {});
      updateMarketOrders();
    })
    .catch(err => {
      addLog(`Error placing sell order: ${err.message}`, 'bad');
    });
}

function addLog(text, className = '') {
  const log = document.getElementById('event-log');
  const entry = document.createElement('div');
  entry.className = 'log-entry' + (className ? ' ' + className : '');
  const time = new Date().toLocaleTimeString('en-US', { hour12: false });
  entry.textContent = `${time} ${text}`;
  log.appendChild(entry);
  // Keep only last 30 entries
  while (log.children.length > 31) { // +1 for header
    log.removeChild(log.children[1]);
  }
  log.scrollTop = log.scrollHeight;
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function getInitiativeIcon(type) {
  const icons = {
    HOUSING: '🏠',
    INDUSTRY: '🏭',
    EDUCATION: '🎓',
    HEALTH: '🏥',
    TAX_CHANGE: '💰',
    TRADE_DEAL: '🤝',
    INNOVATION: '💡',
    POLICY: '📋',
    SECURITY: '🚔',
    GREEN: '🌱',
  };
  return icons[type] || '📌';
}

// ─── Page Load Handler ────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
  const savedSession = checkSavedSession();
  if (savedSession) {
    const loginScreen = document.getElementById('login-screen');
    loginScreen.innerHTML = `
      <div class="pixel-title">CITIES</div>
      <div class="pixel-subtitle">Social Simulator</div>
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

// Handle Enter key on inputs
document.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && document.getElementById('login-screen').style.display !== 'none') {
    startGame();
  }
});
