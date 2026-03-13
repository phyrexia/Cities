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
    this.add.rectangle(0, 0, W, H * 0.45, 0x1a1a3e).setOrigin(0, 0);
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
    this._rebuildBuildings(city.buildings || []);

    // Scale citizens to population
    this._scaleCitizens(city.population?.total || 0);
  }

  _createBuildingSlots(W, H) {
    const groundY = H * 0.58;
    return [
      { x: W * 0.08, y: groundY },
      { x: W * 0.18, y: groundY },
      { x: W * 0.30, y: groundY },
      { x: W * 0.43, y: groundY },
      { x: W * 0.55, y: groundY },
      { x: W * 0.67, y: groundY },
      { x: W * 0.78, y: groundY },
      { x: W * 0.88, y: groundY },
    ];
  }

  _rebuildBuildings(buildings) {
    // Clear existing
    this.buildingGraphics.forEach(g => g.destroy());
    this.buildingGraphics = [];

    buildings.forEach((building, i) => {
      if (i >= this.buildingSlots.length) return;
      const slot = this.buildingSlots[i];
      const scale = 1.5 + (building.level || 1) * 0.3;
      const g = PixelBuilding.draw(this, building.type, slot.x, slot.y - 40, scale);
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
    const colors = [0xFFD700, 0x00FF88, 0x4A9EFF, 0xFF4444, 0xFF88AA];

    // 50 citizens (can show up to 50 at once)
    for (let i = 0; i < 50; i++) {
      const g = this.add.graphics();
      const color = colors[i % colors.length];
      g.fillStyle(color);
      g.fillRect(0, 0, 4, 8); // body
      g.fillStyle(0xFFDDAA);
      g.fillCircle(2, -3, 3); // head

      g.x = Phaser.Math.Between(0, W);
      g.y = walkY + Phaser.Math.Between(-6, 6);
      g.speed = (Math.random() < 0.5 ? 1 : -1) * (0.5 + Math.random() * 1.2);
      g.setVisible(false);

      this.citizens.push(g);
    }

    // Create vehicles (cars on the road)
    this.vehicles = [];
    const roadY = H * 0.58 + 6;
    for (let i = 0; i < 6; i++) {
      const car = this.add.graphics();
      car.fillStyle(0xFF4444);
      car.fillRect(0, 0, 12, 6); // car body
      car.fillStyle(0x222222);
      car.fillRect(2, -2, 4, 3); // window
      car.fillStyle(0x000000);
      car.fillCircle(2, 6, 1.5); // wheel
      car.fillCircle(10, 6, 1.5); // wheel

      car.x = Phaser.Math.Between(0, W);
      car.y = roadY;
      car.speed = (Math.random() < 0.5 ? 1 : -1) * (1.0 + Math.random() * 1.5);
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
