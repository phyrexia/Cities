/**
 * Cities — Social Simulator
 * Main entry point for the Phaser.js 8-bit web client.
 */

let currentHeartbeat = null;
let selectedProposalID = null;
window.gameState = {
  playerID: null,
  cityID: null,
  city: null,
};
const gameState = window.gameState;

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

function updateHUD(city) {
  if (!city) return;
  console.log('[HUD] Updating with city data:', city.name, city.resources);
  document.getElementById('hud-city').textContent = city.name || '—';
  document.getElementById('hud-pop').textContent = (city.population?.total || 0).toLocaleString();
  document.getElementById('hud-happy').textContent = (city.happiness || 0).toFixed(1) + '%';
  document.getElementById('hud-treasury').textContent = (city.treasury || 0).toLocaleString() + ' ¢';
  document.getElementById('hud-tax').textContent = (city.tax_rate || 0).toFixed(1) + '%';
  document.getElementById('hud-round').textContent = city.round || '0';

  // Resources
  const res = city.resources || {};
  const resContainer = document.getElementById('hud-resources');
  resContainer.innerHTML = `
    <span class="resource-badge res-food">FOOD: ${res.food || 0}</span>
    <span class="resource-badge res-metal">METAL: ${res.metal || 0}</span>
    <span class="resource-badge res-materials">MATS: ${res.materials || 0}</span>
    ${res.medicines ? `<span class="resource-badge res-medicines">MEDS: ${res.medicines}</span>` : ''}
  `;
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

// Handle Enter key on inputs
document.addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && document.getElementById('login-screen').style.display !== 'none') {
    startGame();
  }
});
