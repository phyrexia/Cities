/**
 * 8-bit building renderer using Phaser graphics.
 * Each building type has a distinct pixel art shape and color.
 */

const BUILDING_COLORS = {
  house:              { wall: 0xC8A882, roof: 0xCC4444, window: 0x87CEEB },
  affordable_housing: { wall: 0xA0A0A0, roof: 0x886644, window: 0x87CEEB },
  school:             { wall: 0xF5DEB3, roof: 0x4477CC, window: 0xFFFF88 },
  university:         { wall: 0xEEE8AA, roof: 0x8B4513, window: 0xFFFF88 },
  hospital:           { wall: 0xFFFFFF, roof: 0xCC0000, window: 0x87CEEB },
  factory:            { wall: 0x808080, roof: 0x555555, window: 0xFF8800 },
  market:             { wall: 0xFFD700, roof: 0x8B4513, window: 0x87CEEB },
  park:               { wall: 0x228B22, roof: 0x006400, window: null },
  police:             { wall: 0x1a1a6e, roof: 0x000080, window: 0x87CEEB },
  lab:                { wall: 0xE0E0FF, roof: 0x6060AA, window: 0x00FFFF },
  port:               { wall: 0x4682B4, roof: 0x2F4F4F, window: 0x87CEEB },
};

class PixelBuilding {
  /**
   * Draw a building at (x, y) on the given Phaser scene.
   * @param {Phaser.Scene} scene
   * @param {string} type - BuildingType string
   * @param {number} x
   * @param {number} y
   * @param {number} scale - pixel scale multiplier
   */
  static draw(scene, type, x, y, scale = 2) {
    const colors = BUILDING_COLORS[type] || BUILDING_COLORS.house;
    const g = scene.add.graphics();

    const s = scale * 4; // base unit in pixels

    switch (type) {
      case 'house':
      case 'affordable_housing':
        PixelBuilding._drawHouse(g, x, y, s, colors);
        break;
      case 'school':
      case 'university':
        PixelBuilding._drawSchool(g, x, y, s, colors);
        break;
      case 'hospital':
        PixelBuilding._drawHospital(g, x, y, s, colors);
        break;
      case 'factory':
        PixelBuilding._drawFactory(g, x, y, s, colors);
        break;
      case 'market':
        PixelBuilding._drawMarket(g, x, y, s, colors);
        break;
      case 'park':
        PixelBuilding._drawPark(g, x, y, s, colors);
        break;
      case 'police':
        PixelBuilding._drawPolice(g, x, y, s, colors);
        break;
      case 'lab':
        PixelBuilding._drawLab(g, x, y, s, colors);
        break;
      case 'port':
        PixelBuilding._drawPort(g, x, y, s, colors);
        break;
      default:
        PixelBuilding._drawHouse(g, x, y, s, colors);
    }

    return g;
  }

  static _drawHouse(g, x, y, s, c) {
    // Roof (triangle via polygon)
    g.fillStyle(c.roof);
    g.fillTriangle(x, y, x + s * 3, y, x + s * 1.5, y - s * 1.5);
    // Wall
    g.fillStyle(c.wall);
    g.fillRect(x, y, s * 3, s * 2);
    // Door
    g.fillStyle(0x8B4513);
    g.fillRect(x + s, y + s, s, s * 2);
    // Windows
    if (c.window) {
      g.fillStyle(c.window);
      g.fillRect(x + 0.25 * s, y + 0.25 * s, s * 0.75, s * 0.75);
      g.fillRect(x + 2 * s, y + 0.25 * s, s * 0.75, s * 0.75);
    }
  }

  static _drawSchool(g, x, y, s, c) {
    // Main building
    g.fillStyle(c.wall);
    g.fillRect(x, y, s * 5, s * 3);
    // Roof
    g.fillStyle(c.roof);
    g.fillRect(x, y - s * 0.5, s * 5, s * 0.5);
    // Flag
    g.fillStyle(0xFF0000);
    g.fillRect(x + s * 2, y - s * 2, s * 0.2, s * 1.5);
    g.fillRect(x + s * 2.2, y - s * 2, s * 1, s * 0.8);
    // Windows row
    if (c.window) {
      g.fillStyle(c.window);
      for (let i = 0; i < 3; i++) {
        g.fillRect(x + s * (0.5 + i * 1.5), y + s * 0.5, s, s);
      }
    }
    // Door
    g.fillStyle(0x8B4513);
    g.fillRect(x + s * 2, y + s * 1.5, s, s * 1.5);
  }

  static _drawHospital(g, x, y, s, c) {
    g.fillStyle(c.wall);
    g.fillRect(x, y, s * 4, s * 4);
    g.fillStyle(c.roof);
    g.fillRect(x, y - s * 0.5, s * 4, s * 0.5);
    // Red cross
    g.fillStyle(0xFF0000);
    g.fillRect(x + s * 1.5, y + s * 0.5, s, s * 2.5);
    g.fillRect(x + s * 0.5, y + s * 1.5, s * 3, s);
  }

  static _drawFactory(g, x, y, s, c) {
    // Main body
    g.fillStyle(c.wall);
    g.fillRect(x, y + s, s * 5, s * 3);
    // Roof
    g.fillStyle(c.roof);
    g.fillRect(x, y + s - s * 0.3, s * 5, s * 0.3);
    // Chimneys
    g.fillStyle(0x555555);
    g.fillRect(x + s, y - s, s * 0.6, s * 2);
    g.fillRect(x + s * 2.5, y - s * 0.5, s * 0.6, s * 1.5);
    g.fillRect(x + s * 3.5, y - s * 1.2, s * 0.6, s * 2.2);
    // Smoke particles (static yellow-orange dots)
    g.fillStyle(0xFF8800, 0.6);
    g.fillCircle(x + s * 1.3, y - s * 1.2, s * 0.3);
    g.fillStyle(0xFFAA00, 0.4);
    g.fillCircle(x + s * 3.8, y - s * 1.5, s * 0.25);
    // Windows
    if (c.window) {
      g.fillStyle(c.window);
      for (let i = 0; i < 3; i++) {
        g.fillRect(x + s * (0.5 + i * 1.5), y + s * 1.5, s, s * 0.75);
      }
    }
  }

  static _drawMarket(g, x, y, s, c) {
    g.fillStyle(c.wall);
    g.fillRect(x, y, s * 4, s * 3);
    // Awning
    g.fillStyle(0xFF4444);
    g.fillRect(x - s * 0.3, y - s * 0.5, s * 4.6, s * 0.5);
    // Sign
    g.fillStyle(c.roof);
    g.fillRect(x + s * 0.5, y - s * 1.5, s * 3, s);
    // Stands/stalls
    g.fillStyle(0xFFD700);
    g.fillRect(x + s * 0.5, y + s * 2, s, s * 0.5);
    g.fillRect(x + s * 2.5, y + s * 2, s, s * 0.5);
  }

  static _drawPark(g, x, y, s, c) {
    // Grass
    g.fillStyle(0x228B22);
    g.fillRect(x, y, s * 4, s * 3);
    // Trees
    g.fillStyle(0x006400);
    g.fillCircle(x + s, y + s, s * 0.8);
    g.fillCircle(x + s * 3, y + s, s * 0.8);
    g.fillCircle(x + s * 2, y + s * 1.5, s * 1);
    // Trunks
    g.fillStyle(0x8B4513);
    g.fillRect(x + s * 0.8, y + s * 1.5, s * 0.4, s * 0.8);
    g.fillRect(x + s * 2.8, y + s * 1.5, s * 0.4, s * 0.8);
    // Path
    g.fillStyle(0xD2B48C);
    g.fillRect(x + s * 1.8, y + s * 0.5, s * 0.4, s * 2.5);
  }

  static _drawPolice(g, x, y, s, c) {
    g.fillStyle(c.wall);
    g.fillRect(x, y, s * 3, s * 3);
    g.fillStyle(c.roof);
    g.fillRect(x, y - s * 0.4, s * 3, s * 0.4);
    // Blue light on top
    g.fillStyle(0x0000FF);
    g.fillCircle(x + s * 1.5, y - s * 0.7, s * 0.4);
    // Badge
    g.fillStyle(0xFFD700);
    g.fillCircle(x + s * 1.5, y + s, s * 0.5);
    // Door
    g.fillStyle(0x000080);
    g.fillRect(x + s, y + s * 1.5, s, s * 1.5);
  }

  static _drawLab(g, x, y, s, c) {
    g.fillStyle(c.wall);
    g.fillRect(x, y, s * 4, s * 3);
    // Dome
    g.fillStyle(0xB0B0FF);
    g.fillEllipse(x + s * 2, y, s * 2.5, s * 1.5);
    // Windows (glowing cyan)
    g.fillStyle(0x00FFFF, 0.8);
    for (let i = 0; i < 3; i++) {
      g.fillRect(x + s * (0.3 + i * 1.2), y + s * 0.8, s * 0.8, s * 0.8);
    }
  }

  static _drawPort(g, x, y, s, c) {
    // Dock
    g.fillStyle(0x8B6914);
    g.fillRect(x, y + s * 2, s * 6, s);
    // Warehouse
    g.fillStyle(c.wall);
    g.fillRect(x, y, s * 4, s * 2);
    // Crane arm
    g.fillStyle(0x888888);
    g.fillRect(x + s * 4.5, y - s, s * 0.3, s * 3);
    g.fillRect(x + s * 3.5, y - s, s, s * 0.3);
    // Boat (simple)
    g.fillStyle(0x4682B4);
    g.fillEllipse(x + s * 1.5, y + s * 2.8, s * 3, s * 0.8);
    g.fillStyle(0xFFFFFF);
    g.fillRect(x + s * 1.3, y + s * 1.8, s * 0.2, s);
  }
}

window.PixelBuilding = PixelBuilding;
