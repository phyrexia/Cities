package trade

import (
	"fmt"
	"sync"
	"time"

	"github.com/cities/game/internal/city"
	"github.com/google/uuid"
)

// Direction indicates whether we're buying or selling.
type Direction string

const (
	DirectionSell Direction = "sell"
	DirectionBuy  Direction = "buy"
)

// Order represents a pending trade between two cities.
type Order struct {
	ID           string
	FromCityID   string
	ToCityID     string
	ProductID    string
	Quantity     int
	PricePerUnit int64
	Direction    Direction
	Status       string // pending, settled, cancelled
	CreatedAt    time.Time
}

// Engine manages trade orders and settlement between cities.
type Engine struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

// New creates a new trade engine.
func New() *Engine {
	return &Engine{
		orders: make(map[string]*Order),
	}
}

// PlaceOrder creates a new trade order.
func (e *Engine) PlaceOrder(fromCityID, toCityID, productID string, qty int, price int64, dir Direction) (*Order, error) {
	prod := GetByID(productID)
	if prod == nil {
		return nil, fmt.Errorf("product %q not found", productID)
	}
	if qty <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if price <= 0 {
		return nil, fmt.Errorf("price must be positive")
	}

	order := &Order{
		ID:           uuid.NewString(),
		FromCityID:   fromCityID,
		ToCityID:     toCityID,
		ProductID:    productID,
		Quantity:     qty,
		PricePerUnit: price,
		Direction:    dir,
		Status:       "pending",
		CreatedAt:    time.Now(),
	}

	e.mu.Lock()
	e.orders[order.ID] = order
	e.mu.Unlock()
	return order, nil
}

// CancelOrder cancels a pending order.
func (e *Engine) CancelOrder(orderID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	order, ok := e.orders[orderID]
	if !ok {
		return fmt.Errorf("order %q not found", orderID)
	}
	if order.Status != "pending" {
		return fmt.Errorf("order %q is already %s", orderID, order.Status)
	}
	order.Status = "cancelled"
	return nil
}

// TradeResult holds the result of a settled trade.
type TradeResult struct {
	OrderID      string
	ProductID    string
	Quantity     int
	TotalValue   int64
	FromCityID   string
	ToCityID     string
}

// SettleAll processes all pending orders and returns results.
// Cities map is used to transfer treasury and validate products.
func (e *Engine) SettleAll(cities map[string]*city.City) []TradeResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	var results []TradeResult
	for _, order := range e.orders {
		if order.Status != "pending" {
			continue
		}
		fromCity := cities[order.FromCityID]
		toCity := cities[order.ToCityID]
		if fromCity == nil || toCity == nil {
			order.Status = "cancelled"
			continue
		}

		totalValue := int64(order.Quantity) * order.PricePerUnit

		// Seller must have the product
		if order.Direction == DirectionSell {
			if !hasProduct(fromCity, order.ProductID) {
				order.Status = "cancelled"
				continue
			}
			// Buyer must have the treasury
			if toCity.Treasury < totalValue {
				order.Status = "cancelled"
				continue
			}
			// Transfer
			toCity.Treasury -= totalValue
			fromCity.Treasury += totalValue
		} else {
			// BUY: buyer pays, seller receives
			if !hasProduct(toCity, order.ProductID) {
				order.Status = "cancelled"
				continue
			}
			if fromCity.Treasury < totalValue {
				order.Status = "cancelled"
				continue
			}
			fromCity.Treasury -= totalValue
			toCity.Treasury += totalValue
		}

		// Ensure cities are connected as trade partners
		addTradeRoute(fromCity, toCity.ID)
		addTradeRoute(toCity, fromCity.ID)

		order.Status = "settled"
		results = append(results, TradeResult{
			OrderID:    order.ID,
			ProductID:  order.ProductID,
			Quantity:   order.Quantity,
			TotalValue: totalValue,
			FromCityID: order.FromCityID,
			ToCityID:   order.ToCityID,
		})
	}
	return results
}

// PendingOrders returns all pending orders for a city.
func (e *Engine) PendingOrders(cityID string) []*Order {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*Order
	for _, o := range e.orders {
		if o.Status == "pending" && (o.FromCityID == cityID || o.ToCityID == cityID) {
			result = append(result, o)
		}
	}
	return result
}

func hasProduct(c *city.City, productID string) bool {
	for _, p := range c.Products {
		if p == productID {
			return true
		}
	}
	return false
}

func addTradeRoute(c *city.City, partnerID string) {
	for _, id := range c.TradeRoutes {
		if id == partnerID {
			return
		}
	}
	c.TradeRoutes = append(c.TradeRoutes, partnerID)
}
