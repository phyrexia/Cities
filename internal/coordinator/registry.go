package coordinator

import (
	"fmt"
	"sync"
	"time"

	"github.com/cities/game/internal/city"
	"github.com/google/uuid"
)

// Player represents a connected player/mayor.
type Player struct {
	ID        string
	Name      string
	CityID    string
	Token     string
	ConnectedAt time.Time
	LastSeen  time.Time
}

// Registry manages all cities and players in the game.
type Registry struct {
	mu      sync.RWMutex
	cities  map[string]*city.City
	players map[string]*Player
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		cities:  make(map[string]*city.City),
		players: make(map[string]*Player),
	}
}

// RegisterPlayer creates a new player and their city.
func (r *Registry) RegisterPlayer(playerName, cityName string) (*Player, *city.City, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	playerID := uuid.NewString()
	token := uuid.NewString()

	c := city.New(cityName, playerID, playerName)

	player := &Player{
		ID:          playerID,
		Name:        playerName,
		CityID:      c.ID,
		Token:       token,
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
	}

	r.players[playerID] = player
	r.cities[c.ID] = c

	return player, c, nil
}

// GetCity retrieves a city by ID.
func (r *Registry) GetCity(cityID string) (*city.City, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.cities[cityID]
	if !ok {
		return nil, fmt.Errorf("city %q not found", cityID)
	}
	return c, nil
}

// GetPlayer retrieves a player by ID.
func (r *Registry) GetPlayer(playerID string) (*Player, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %q not found", playerID)
	}
	return p, nil
}

// GetPlayerByToken retrieves a player by session token.
func (r *Registry) GetPlayerByToken(token string) (*Player, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.players {
		if p.Token == token {
			return p, nil
		}
	}
	return nil, fmt.Errorf("invalid token")
}

// AllCities returns a snapshot of all cities.
func (r *Registry) AllCities() []*city.City {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cities := make([]*city.City, 0, len(r.cities))
	for _, c := range r.cities {
		cities = append(cities, c)
	}
	return cities
}

// AllPlayers returns all registered players.
func (r *Registry) AllPlayers() []*Player {
	r.mu.RLock()
	defer r.mu.RUnlock()
	players := make([]*Player, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, p)
	}
	return players
}

// UpdateCity applies an update function to a city (thread-safe).
func (r *Registry) UpdateCity(cityID string, fn func(*city.City)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cities[cityID]
	if !ok {
		return fmt.Errorf("city %q not found", cityID)
	}
	fn(c)
	c.UpdatedAt = time.Now()
	return nil
}

// CitiesMap returns a copy of the cities map (for trade engine).
func (r *Registry) CitiesMap() map[string]*city.City {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := make(map[string]*city.City, len(r.cities))
	for k, v := range r.cities {
		m[k] = v
	}
	return m
}

// PlayerCount returns total number of players.
func (r *Registry) PlayerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.players)
}

// UpdatePlayerActivity marks a player as recently active.
func (r *Registry) UpdatePlayerActivity(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.players[playerID]; ok {
		p.LastSeen = time.Now()
	}
}
