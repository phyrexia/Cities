# 🏙️ Cities — Social Simulator

A hyper-dynamic 8-bit social simulator where you manage a city's growth, economy, and crises. Powered by **OpenRouter (Gemini 2.5)** for AI-driven political initiatives and social events.

## 🚀 Features

- **🧠 AI-Driven Governance**: Every 30 seconds, an AI council (Gemini 2.5) generates unique initiatives based on your city's current state (unemployment, happiness, resources).
- **📦 Quantitative Resource System**: Track and manage **Food, Metal, Materials, and Medicines**.
- **⚙️ Deep Economy Engine**: Factories produce materials, markets generate food, and labs develop medicines. Watch out for population consumption!
- **🏚️ Ruins & Recovery**: Cities can collapse into ruins due to bankruptcy or mass emigration, but they can be reclaimed by new mayors.
- **🌐 Real-time Web HUD**: A responsive Phaser.js client with a full-screen HUD and interactive proposal cards.

## 🛠️ Tech Stack

- **Backend**: Go (Golang)
- **Frontend**: Phaser.js (8-bit aesthetics)
- **Database**: PostgreSQL
- **Caching/PubSub**: Redis
- **AI Integration**: OpenRouter API
- **Infrastructure**: Docker & Docker Compose

## 🏁 Getting Started

### Prerequisites

- [Docker](https://www.docker.com/) & Docker Compose
- [OpenRouter API Key](https://openrouter.ai/)

### Setup

1. **Clone the repository**:
   ```bash
   git clone <repository-url>
   cd Cities
   ```

2. **Configure Environment**:
   Copy the example environment file and add your OpenRouter API key.
   ```bash
   cp .env.example .env
   ```
   Edit `.env` and set `OPENROUTER_API_KEY`.

3. **Launch the Game**:
   ```bash
   docker-compose up -d --build
   ```

4. **Play**:
   Open [http://localhost:8080](http://localhost:8080) in your browser.

## 🎮 Game Mechanics

- **Heartbeat**: Every 30 seconds, a "Heartbeat" occurs. You will receive 3-4 initiatives from the AI council.
- **Initiatives**: Choose one initiative per heartbeat. Each has unique effects on population, happiness, treasury, and resources.
- **Resources**:
  - 🍞 **Food**: Consumed by population. Produced by Markets.
  - 🏗️ **Materials**: Used for infrastructure. Produced by Factories.
  - ⛓️ **Metal**: High-value trade resource. Produced by Factories.
  - 💊 **Medicines**: Boosts health level. Produced by Research Labs.

## 📄 License

MIT License. See [LICENSE](LICENSE) for details (if applicable).
