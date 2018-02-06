package transactionserver

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// UserDatabase holds all of the supported database commands
type UserDatabase interface {
	AddFunds(string, decimal.Decimal) error
	GetFunds(string) (decimal.Decimal, error)
	RemoveFunds(string, decimal.Decimal) error

	AddStock(user string, stock string, shares int) error
	GetStock(user string, stock string) (int, error)
	RemoveStock(user string, stock string, amount int) error

	PushBuy(user string, stock string, cost decimal.Decimal, shares int) error
	PopBuy(user string) (stock string, cost decimal.Decimal, shares int, err error)
	PushSell(user string, stock string, cost decimal.Decimal, shares int) error
	PopSell(user string) (stock string, cost decimal.Decimal, shares int, err error)

	AddBuyTrigger(user string, stock string, trigger *Trigger) error
	GetBuyTrigger(user string, stock string) (*Trigger, error)
	RemoveBuyTrigger(user string, stock string) (*Trigger, error)

	AddSellTrigger(user string, stock string, trigger *Trigger) error
	GetSellTrigger(user string, stock string) (*Trigger, error)
	RemoveSellTrigger(user string, stock string) (*Trigger, error)
}

// RedisDatabase holds the address of the redisDB
type RedisDatabase struct {
	addr string
}

// AddSellTrigger adds a sell trigger to the redisDB
func (u RedisDatabase) AddSellTrigger(user string, stock string, trigger *Trigger) error {
	panic("implement me")
}

// GetSellTrigger gets any available triggers that a user has already set
func (u RedisDatabase) GetSellTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

// RemoveSellTrigger removes any sell trigger corresponding to a stock.
// This may be unset, or set
func (u RedisDatabase) RemoveSellTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

// AddBuyTrigger adds a trigger for a user, for a specified stock
func (u RedisDatabase) AddBuyTrigger(user string, stock string, trigger *Trigger) error {
	panic("implement me")
}

// RemoveBuyTrigger removes a users buy trigger for the corresponding stock
func (u RedisDatabase) RemoveBuyTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

// GetBuyTrigger gets a user's trigger for the specified stock, if one exists
func (u RedisDatabase) GetBuyTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

// GetStock returns the users available balance of said stock
func (u RedisDatabase) GetStock(user string, stock string) (int, error) {
	panic("implement me")
}

// RemoveStock removes int stocks from the users account
func (u RedisDatabase) RemoveStock(user string, stock string, amount int) error {
	panic("implement me")
}

// PushSell adds a record of the users requested sell to their account
func (u RedisDatabase) PushSell(user string, stock string, cost decimal.Decimal, shares int) error {
	panic("implement me")
}

// PopSell removes a users most recent requested sell
func (u RedisDatabase) PopSell(user string) (stock string, cost decimal.Decimal, shares int, err error) {
	panic("implement me")
}

// PushBuy adds a record of the users requested buy to their account
func (u RedisDatabase) PushBuy(user string, stock string, cost decimal.Decimal, shares int) error {
	// Expires in 60s
	panic("implement me")
}

// PopBuy removes a users most recent requested buy
func (u RedisDatabase) PopBuy(user string) (stock string, cost decimal.Decimal, shares int, err error) {
	panic("implement me")
}

// AddFunds adds amount dollars to the user account
func (u RedisDatabase) AddFunds(user string, amount decimal.Decimal) error {
	fmt.Print(user)
	fmt.Print(amount)
	return nil
}

// GetFunds returns the amount of available funds in a users account
func (u RedisDatabase) GetFunds(user string) (decimal.Decimal, error) {
	return decimal.NewFromFloat(0), nil
}

// RemoveFunds remove n funds from the user's account
func (u RedisDatabase) RemoveFunds(user string, amount decimal.Decimal) error {
	return nil
}

// AddStock adds shares to the user account
func (u RedisDatabase) AddStock(user string, stock string, shares int) error {
	return nil
}
