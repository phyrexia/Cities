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
}

// ─── Phaser Game ─────────────────────────────────────────────────────────────

function initPhaser() {
  const config = {
    type: Phaser.CANVAS,
    width: Math.min(window.innerWidth, 800),
    height: 400,
    canvas: document.getElementById('phaser-canvas'),
    backgroundColor: '#0a0a0f',
    pixelArt: true,
    antialias: false,
    scene: [GameScene],
  };

  window.phaserGame = new Phaser.Game(config);
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
        updateHUD(update.city);
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

// ─── Event Log ───────────────────────────────────────────────────────────────

function animateCounter(elementId, fromValue, toValue, duration = 600) {
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

function renderDelta(current, previous) {
  const delta = current - previous;
  if (delta === 0) return '—';
  const arrow = delta > 0 ? '↑' : '↓';
  const color = delta > 0 ? '#00FF88' : '#FF4444';
  const sign = delta > 0 ? '+' : '';
  return `<span style="color: ${color}; margin-left: 8px;">${arrow}${sign}${delta}</span>`;
}

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

  // Update previous values
  RESOURCE_CATALOG.forEach(r => {
    prevResources[r.key] = resources[r.key] || 0;
  });
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
}

function renderMyProducts(resources) {
  // Will implement in Task 12
  const panel = document.getElementById('my-products-list');
  panel.innerHTML = '<div style="color: #666; text-align: center;">Loading...</div>';
}

async function updateMarketOrders() {
  // Will implement in Task 11
  const panel = document.getElementById('market-orders-list');
  panel.innerHTML = '<div style="color: #666; text-align: center;">Loading market...</div>';
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
