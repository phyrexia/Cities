package coordinator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cities/game/internal/trade"
)

func TestHandleListOrders(t *testing.T) {
	s := NewServer()

	// Place a test order
	order, err := s.tradeEngine.PlaceOrder("city1", "city2", "wood", 100, 50, trade.DirectionSell)
	if err != nil {
		t.Fatalf("Failed to place order: %v", err)
	}

	// Test GET /api/trade/orders
	req := httptest.NewRequest(http.MethodGet, "/api/trade/orders", nil)
	w := httptest.NewRecorder()
	s.handleListOrders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var orders []*trade.Order
	if err := json.NewDecoder(w.Body).Decode(&orders); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(orders) == 0 {
		t.Error("Expected at least one order")
	}

	// Verify the order we placed is in the list
	found := false
	for _, o := range orders {
		if o.ID == order.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Order not found in list")
	}
}

func TestHandleListOrdersMethodNotAllowed(t *testing.T) {
	s := NewServer()

	// Test POST (should be rejected)
	req := httptest.NewRequest(http.MethodPost, "/api/trade/orders", bytes.NewBufferString("{}"))
	w := httptest.NewRecorder()
	s.handleListOrders(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleListOrdersEmpty(t *testing.T) {
	s := NewServer()

	// Test GET with no orders
	req := httptest.NewRequest(http.MethodGet, "/api/trade/orders", nil)
	w := httptest.NewRecorder()
	s.handleListOrders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var orders []*trade.Order
	if err := json.NewDecoder(w.Body).Decode(&orders); err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	// Should be nil or empty slice
	if orders == nil && len(orders) > 0 {
		t.Error("Expected empty order list")
	}
}
