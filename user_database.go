package transactionserver

import (
	"fmt"

	"github.com/shopspring/decimal"
)

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

type RedisDatabase struct {
	addr string
}

func (u RedisDatabase) AddSellTrigger(user string, stock string, trigger *Trigger) error {
	panic("implement me")
}

func (u RedisDatabase) GetSellTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

func (u RedisDatabase) RemoveSellTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

func (u RedisDatabase) AddBuyTrigger(user string, stock string, trigger *Trigger) error {
	panic("implement me")
}

func (u RedisDatabase) RemoveBuyTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

func (u RedisDatabase) GetBuyTrigger(user string, stock string) (*Trigger, error) {
	panic("implement me")
}

func (u RedisDatabase) SetBuyTrigger(user string, stock string, trigger *Trigger) error {
	panic("implement me")
}

func (u RedisDatabase) SetBuyTriggerActive(user string, stock string) (decimal.Decimal, error) {
	panic("implement me")
}

func (u RedisDatabase) GetStock(user string, stock string) (int, error) {
	panic("implement me")
}

func (u RedisDatabase) RemoveStock(user string, stock string, amount int) error {
	panic("implement me")
}

func (u RedisDatabase) PushSell(user string, stock string, cost decimal.Decimal, shares int) error {
	panic("implement me")
}

func (u RedisDatabase) PopSell(user string) (stock string, cost decimal.Decimal, shares int, err error) {
	panic("implement me")
}

func (u RedisDatabase) PushBuy(user string, stock string, cost decimal.Decimal, shares int) error {
	// Expires in 60s
	panic("implement me")
}

func (u RedisDatabase) PopBuy(user string) (stock string, cost decimal.Decimal, shares int, err error) {
	panic("implement me")
}

func (u RedisDatabase) AddFunds(user string, amount decimal.Decimal) error {
	fmt.Print(user)
	fmt.Print(amount)
	return nil
}

func (u RedisDatabase) GetFunds(user string) (decimal.Decimal, error) {
	return decimal.NewFromFloat(0), nil
}

func (u RedisDatabase) RemoveFunds(user string, amount decimal.Decimal) error {
	return nil
}

func (u RedisDatabase) AddStock(user string, stock string, shares int) error {
	return nil
}
