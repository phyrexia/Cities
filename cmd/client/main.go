package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cities/game/internal/tui"
)

func main() {
	var (
		serverURL  = flag.String("server", "http://localhost:8080", "Coordinator server URL")
		playerName = flag.String("player", "", "Your player name (mayor name)")
		cityName   = flag.String("city", "", "Your city name")
		playerID   = flag.String("player-id", "", "Existing player ID (skip registration)")
		cityID     = flag.String("city-id", "", "Existing city ID (skip registration)")
	)
	flag.Parse()

	if *playerName == "" {
		fmt.Print("Enter your name (mayor): ")
		fmt.Scan(playerName)
	}
	if *cityName == "" {
		fmt.Print("Enter your city name: ")
		fmt.Scan(cityName)
	}

	var pID, cID string

	if *playerID != "" && *cityID != "" {
		pID = *playerID
		cID = *cityID
		log.Printf("Reconnecting as player %s, city %s", pID, cID)
	} else {
		// Register with coordinator
		log.Printf("Registering %s as mayor of %s...", *playerName, *cityName)
		p, c, err := register(*serverURL, *playerName, *cityName)
		if err != nil {
			log.Fatalf("Registration failed: %v", err)
		}
		pID = p
		cID = c
		log.Printf("Registered! Player ID: %s  City ID: %s", pID, cID)
		log.Printf("(Use --player-id %s --city-id %s to reconnect)", pID, cID)
	}

	model := tui.New(pID, cID, *serverURL)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func register(serverURL, playerName, cityName string) (playerID, cityID string, err error) {
	body, _ := json.Marshal(map[string]string{
		"player_name": playerName,
		"city_name":   cityName,
	})
	resp, err := http.Post(serverURL+"/api/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("server returned %d", resp.StatusCode)
	}
	var result struct {
		PlayerID string `json:"player_id"`
		CityID   string `json:"city_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode: %w", err)
	}
	return result.PlayerID, result.CityID, nil
}
