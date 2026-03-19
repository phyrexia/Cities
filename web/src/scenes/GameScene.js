/**
 * GameScene — Main 8-bit city view.
 * Shows the player's city with pixel art buildings, citizens, and stats.
 */
class GameScene extends Phaser.Scene {
  constructor() {
    super({ key: 'GameScene' });
    this.cityData = null;
    this.buildingGraphics = [];
    this.citizens = [];
    this.round = 0;
  }

  create() {
    const W = this.scale.width;
    const H = this.scale.height;

    // Sky gradient background
    this.skyRect = this.add.rectangle(0, 0, W, H * 0.45, 0x1a1a3e).setOrigin(0, 0);
    this.add.rectangle(0, H * 0.45, W, H * 0.15, 0x2d5a27).setOrigin(0, 0); // grass strip
    this.add.rectangle(0, H * 0.60, W, H * 0.40, 0x1a1208).setOrigin(0, 0); // ground

    // Pixel stars in sky
    const starGfx = this.add.graphics();
    starGfx.fillStyle(0xFFFFFF, 0.6);
    for (let i = 0; i < 50; i++) {
      starGfx.fillRect(
        Phaser.Math.Between(0, W),
        Phaser.Math.Between(0, H * 0.4),
        2, 2
      );
    }

    // Ground road
    const roadGfx = this.add.graphics();
    roadGfx.fillStyle(0x444444);
    roadGfx.fillRect(0, H * 0.58, W, 16);
    // Road dashes
    roadGfx.fillStyle(0xFFFFFF);
    for (let x = 0; x < W; x += 40) {
      roadGfx.fillRect(x, H * 0.58 + 6, 20, 4);
    }

    // City name banner
    this.cityLabel = this.add.text(W / 2, 24, 'LOADING...', {
      fontSize: '20px',
      fontFamily: 'Courier New',
      color: '#FFD700',
      stroke: '#000000',
      strokeThickness: 4,
    }).setOrigin(0.5, 0);

    // Round counter
    this.roundLabel = this.add.text(16, 16, 'ROUND 0', {
      fontSize: '12px',
      fontFamily: 'Courier New',
      color: '#4A9EFF',
    });

    // Building slots (positions where buildings can appear)
    this.buildingSlots = this._createBuildingSlots(W, H);

    // Citizen sprites (colored walking dots)
    this._createCitizens(W, H);

    // Confetti pool (pre-created, hidden until needed)
    this._createConfettiPool(W, H);

    // Register for city updates
    if (window.citiesWS) {
      window.citiesWS.on('city_update', (data) => {
        if (data && data.city) {
          this.updateCity(data.city);
        }
      });
      window.citiesWS.on('world_event', (data) => {
        if (data) {
          this._showWorldEvent(data.description);
        }
      });
    }

    // Floating event text pool
    this.eventTextPool = [];
  }

  update(time, delta) {
    // Animate citizens walking
    this.citizens.forEach((citizen, i) => {
      citizen.x += citizen.speed;
      if (citizen.x > this.scale.width + 20) {
        citizen.x = -20;
      }
      if (citizen.x < -20) {
        citizen.x = this.scale.width + 20;
      }
    });

    // Animate vehicles on road
    if (this.vehicles) {
      this.vehicles.forEach((vehicle, i) => {
        vehicle.x += vehicle.speed;
        if (vehicle.x > this.scale.width + 20) {
          vehicle.x = -20;
        }
        if (vehicle.x < -20) {
          vehicle.x = this.scale.width + 20;
        }
      });
    }
  }

  updateCity(city) {
    this.cityData = city;
    this.cityLabel.setText(city.name.toUpperCase());
    this.round = city.round || 0;
    this.roundLabel.setText(`ROUND ${this.round}`);

    // Update HUD
    document.getElementById('hud-city').textContent = city.name;
    document.getElementById('hud-pop').textContent = (city.population?.total || 0).toLocaleString();

    const happy = city.happiness || 0;
    const happyEl = document.getElementById('hud-happy');
    happyEl.textContent = happy.toFixed(0) + '%';
    happyEl.className = 'hud-value ' + (happy >= 60 ? 'good' : happy < 30 ? 'bad' : '');

    const treasury = city.treasury || 0;
    document.getElementById('hud-treasury').textContent = treasury.toLocaleString() + ' ¢';
    document.getElementById('hud-tax').textContent = (city.tax_rate || 0).toFixed(1) + '%';
    document.getElementById('hud-round').textContent = this.round;

    // Rebuild buildings on screen
    this._rebuildBuildings(city.buildings || [], city.stats?.pollution_level || 0);

    // Scale citizens to population
    this._scaleCitizens(city.population?.total || 0);

    // ── Reactive visuals ──────────────────────────────────────────────
    const W = this.scale.width;
    const H = this.scale.height;

    // Ambient sky color based on happiness
    let skyColor;
    if (happy > 70) skyColor = 0x4488cc;      // bright blue
    else if (happy > 40) skyColor = 0x556677;  // grey-blue
    else skyColor = 0x333344;                   // dark stormy
    this.skyRect.setFillStyle(skyColor);

    // Citizen speed based on happiness (scale stored baseSpeed, no re-randomization)
    const speedMultiplier = happy > 70 ? 1.5 : happy > 40 ? 1.0 : 0.5;
    this.citizens.forEach(c => {
      c.speed = c.baseSpeed * speedMultiplier;
    });

    // Protest visuals when any faction <25
    if (this.protestGroup) { this.protestGroup.destroy(); this.protestGroup = null; }
    const factions = city.factions;
    if (factions) {
      const lowFaction = factions.workers < 25 || factions.business < 25 || factions.families < 25 || (factions.greens_active && factions.greens < 25);
      if (lowFaction) {
        this.protestGroup = this.add.graphics();
        this.protestGroup.setDepth(10);
        const px = W * 0.4;
        const py = H * 0.55;
        // Protest sign
        this.protestGroup.fillStyle(0xFF4444);
        this.protestGroup.fillRect(px, py - 20, 2, 15);
        this.protestGroup.fillRect(px - 8, py - 25, 18, 10);
        // Group of angry dots
        for (let i = 0; i < 8; i++) {
          this.protestGroup.fillStyle(0xFF4444);
          this.protestGroup.fillCircle(px - 15 + i * 5, py - 2 + Math.random() * 6, 3);
        }
      }
    }

    // Festival confetti when any faction >80 (pool-based — no create/destroy per update)
    if (factions && this.confettiPool) {
      const highFaction = factions.workers > 80 || factions.business > 80 || factions.families > 80 || (factions.greens_active && factions.greens > 80);
      this.confettiPool.forEach(dot => dot.setVisible(highFaction));
    }
  }

  _createBuildingSlots(W, H) {
    const groundY = H * 0.58;
    // Front row (road level)
    const slots = [
      { x: W * 0.05, y: groundY },
      { x: W * 0.15, y: groundY },
      { x: W * 0.26, y: groundY },
      { x: W * 0.37, y: groundY },
      { x: W * 0.48, y: groundY },
      { x: W * 0.59, y: groundY },
      { x: W * 0.70, y: groundY },
      { x: W * 0.81, y: groundY },
      { x: W * 0.91, y: groundY },
    ];
    // Back row (behind, slightly higher — for cities with 10+ buildings)
    const backY = groundY - H * 0.12;
    for (let i = 0; i < 6; i++) {
      slots.push({ x: W * (0.10 + i * 0.15), y: backY });
    }
    return slots;
  }

  _rebuildBuildings(buildings, pollution) {
    // Clear existing
    this.buildingGraphics.forEach(g => g.destroy());
    this.buildingGraphics = [];

    buildings.forEach((building, i) => {
      if (i >= this.buildingSlots.length) return;
      const slot = this.buildingSlots[i];
      const scale = 1.5 + (building.level || 1) * 0.3;
      const g = PixelBuilding.draw(this, building.type, slot.x, slot.y - 40, scale, pollution);
      this.buildingGraphics.push(g);

      // Building name label
      const label = this.add.text(slot.x + 24, slot.y + 8, building.name.substring(0, 12), {
        fontSize: '8px',
        fontFamily: 'Courier New',
        color: '#aaaaaa',
      }).setOrigin(0.5, 0);
      this.buildingGraphics.push(label);
    });
  }

  _createCitizens(W, H) {
    const walkY = H * 0.60;
    const colors = [0x4A9EFF, 0x4A9EFF, 0xFFD700, 0x00FF88, 0x00FF88, 0x88FF88];

    // 50 citizens (can show up to 50 at once) — 2x larger for visibility
    for (let i = 0; i < 50; i++) {
      const g = this.add.graphics();
      const color = colors[i % colors.length];
      g.fillStyle(color);
      g.fillRect(0, 0, 8, 16); // body (2x larger)
      g.fillStyle(0xFFDDAA);
      g.fillCircle(4, -6, 6); // head (2x larger)

      g.x = Phaser.Math.Between(0, W);
      g.y = walkY + Phaser.Math.Between(-8, 8);
      g.baseSpeed = (Math.random() < 0.5 ? 1 : -1) * (1.0 + Math.random() * 2.0);
      g.speed = g.baseSpeed;
      g.setDepth(0);
      g.setVisible(false);

      this.citizens.push(g);
    }

    // Create vehicles (cars on the road) — 2x larger
    this.vehicles = [];
    const roadY = H * 0.58 + 6;
    for (let i = 0; i < 6; i++) {
      const car = this.add.graphics();
      car.fillStyle(0xFF4444);
      car.fillRect(0, 0, 24, 12); // car body (2x larger)
      car.fillStyle(0x222222);
      car.fillRect(4, -4, 8, 6); // window (2x larger)
      car.fillStyle(0x000000);
      car.fillCircle(4, 12, 3); // wheel (2x larger)
      car.fillCircle(20, 12, 3); // wheel (2x larger)

      car.x = Phaser.Math.Between(0, W);
      car.y = roadY;
      car.speed = (Math.random() < 0.5 ? 1 : -1) * (2.0 + Math.random() * 3.0); // 2x faster
      car.setDepth(-1); // Behind citizens
      car.setVisible(false);

      this.vehicles.push(car);
    }
  }

  _scaleCitizens(totalPop) {
    // 1 citizen per 20 population (more aggressive), up to 50
    const visibleCount = Math.min(50, Math.floor(totalPop / 20));
    this.citizens.forEach((c, i) => {
      c.setVisible(i < visibleCount);
    });

    // Show vehicles based on economic activity
    const visibleVehicles = Math.min(6, Math.floor(totalPop / 100));
    if (this.vehicles) {
      this.vehicles.forEach((v, i) => {
        v.setVisible(i < visibleVehicles);
      });
    }
  }

  _createConfettiPool(W, H) {
    this.confettiPool = [];
    const confettiColors = [0xFFD700, 0xFF4444, 0x00FF88, 0x4A9EFF, 0xFF88AA];
    for (let i = 0; i < 15; i++) {
      const dot = this.add.graphics();
      dot.fillStyle(confettiColors[i % confettiColors.length], 0.8);
      dot.fillCircle(0, 0, 2);
      dot.x = Phaser.Math.Between(0, W);
      dot.y = Phaser.Math.Between(H * 0.2, H * 0.5);
      dot.setVisible(false);
      this.confettiPool.push(dot);
      this.tweens.add({ targets: dot, y: dot.y + 30, alpha: 0, duration: 2000 + Math.random() * 1000, repeat: -1, yoyo: true });
    }
  }

  _showWorldEvent(text) {
    const label = this.add.text(this.scale.width / 2, 80, text, {
      fontSize: '12px',
      fontFamily: 'Courier New',
      color: '#FFD700',
      backgroundColor: '#00000099',
      padding: { x: 8, y: 4 },
    }).setOrigin(0.5, 0).setAlpha(0);

    this.tweens.add({
      targets: label,
      alpha: 1,
      duration: 300,
      yoyo: true,
      hold: 2000,
      onComplete: () => label.destroy(),
    });
  }
}
