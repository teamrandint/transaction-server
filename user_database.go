package transactionserver

import (
	"fmt"
	"github.com/shopspring/decimal"
)

type UserDatabase interface {
	AddFunds(string, decimal.Decimal) error
	GetFunds(string) (decimal.Decimal, error)
	RemoveFunds(string, decimal.Decimal) error
	PushBuy(user string, stock string, cost decimal.Decimal, shares int) error
	PopBuy(user string) (stock string, cost decimal.Decimal, shares int, err error)
	AddStock(user string, stock string, shares int) error
}

type RedisDatabase struct {
	addr string
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