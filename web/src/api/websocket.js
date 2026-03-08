/**
 * WebSocket client for Cities coordinator.
 * Handles connection, reconnection, and message routing.
 */
class CitiesWS {
  constructor(serverURL, playerID, cityID) {
    this.serverURL = serverURL;
    this.playerID = playerID;
    this.cityID = cityID;
    this.ws = null;
    this.listeners = {};
    this.reconnectDelay = 1000;
    this.maxReconnectDelay = 30000;
    this.connected = false;
  }

  connect() {
    const wsURL = this.serverURL.replace(/^http/, 'ws') +
      `/ws?player_id=${this.playerID}&city_id=${this.cityID}`;
    console.log('[WS] Connecting to', wsURL);

    this.ws = new WebSocket(wsURL);

    this.ws.onopen = () => {
      console.log('[WS] Connected');
      this.connected = true;
      this.reconnectDelay = 1000;
      this._emit('connected');
    };

    this.ws.onclose = () => {
      console.log('[WS] Disconnected — reconnecting in', this.reconnectDelay, 'ms');
      this.connected = false;
      this._emit('disconnected');
      setTimeout(() => this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    };

    this.ws.onerror = (err) => {
      console.error('[WS] Error:', err);
      this._emit('error', err);
    };

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        this._emit(msg.type, msg.payload);
        this._emit('message', msg);
      } catch (e) {
        console.error('[WS] Parse error:', e);
      }
    };
  }

  on(event, callback) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(callback);
    return this;
  }

  off(event, callback) {
    if (this.listeners[event]) {
      this.listeners[event] = this.listeners[event].filter(cb => cb !== callback);
    }
  }

  _emit(event, data) {
    (this.listeners[event] || []).forEach(cb => cb(data));
  }

  send(type, payload) {
    if (!this.connected) {
      console.warn('[WS] Not connected — dropping message', type);
      return;
    }
    this.ws.send(JSON.stringify({ type, payload }));
  }

  submitDecision(heartbeatID, initiativeID) {
    this.send('decision', {
      heartbeat_id: heartbeatID,
      initiative_id: initiativeID
    });
  }
}

// Global instance
window.citiesWS = null;
