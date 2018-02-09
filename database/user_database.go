package database

import (
	"fmt"
	"seng468/transaction-server/trigger"

	"github.com/garyburd/redigo/redis"

	"github.com/shopspring/decimal"
)

// UserDatabase holds all of the supported database commands
type UserDatabase interface {
	GetUserInfo(user string) (info string, err error)

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

	AddBuyTrigger(user string, stock string, t *triggers.Trigger) error
	GetBuyTrigger(user string, stock string) (*triggers.Trigger, error)
	RemoveBuyTrigger(user string, stock string) (*triggers.Trigger, error)

	AddSellTrigger(user string, stock string, t *triggers.Trigger) error
	GetSellTrigger(user string, stock string) (*triggers.Trigger, error)
	RemoveSellTrigger(user string, stock string) (*triggers.Trigger, error)
}

// RedisDatabase holds the address of the redisDB
type RedisDatabase struct {
	Addr string
	Port string
}

func (u RedisDatabase) getConn() redis.Conn {
	c, err := redis.Dial(u.Addr, u.Port)
	if err != nil {
		panic(err)
	}
	return c
}

// GetUserInfo returns all of a users information in the database
func (u RedisDatabase) GetUserInfo(user string) (info string, err error) {
	c := u.getConn()
	c.Send("MULTI")
	c.Send("GET", user+":Balance")
	c.Send("GET", user+":Stocks")
	c.Send("GET", user+":SellOrders")
	c.Send("GET", user+":BuyOrders")
	c.Send("GET", user+":SellTriggers")
	c.Send("GET", user+":BuyTriggers")
	c.Send("GET", user+":BalanceReserve")
	c.Send("GET", user+":StocksReserve")
	c.Send("GET", user+":History")
	r, err := c.Do("EXEC")
	if err != nil {
		return "", err
	}

	c.Close()
	return fmt.Sprintf("%v", r), err
}

// AddSellTrigger adds a sell trigger to the redisDB
func (u RedisDatabase) AddSellTrigger(user string, stock string, t *triggers.Trigger) error {
	panic("Not implemented")
	c := u.getConn()
	_, err := c.Do("APPPEND", user+":SellTriggers")
	c.Close()
	return err
}

// GetSellTrigger gets any available triggers that a user has already set
func (u RedisDatabase) GetSellTrigger(user string, stock string) (*triggers.Trigger, error) {
	panic("implement me")
}

// RemoveSellTrigger removes any sell trigger corresponding to a stock.
// This may be unset, or set
func (u RedisDatabase) RemoveSellTrigger(user string, stock string) (*triggers.Trigger, error) {
	panic("implement me")
}

// AddBuyTrigger adds a trigger for a user, for a specified stock
func (u RedisDatabase) AddBuyTrigger(user string, stock string, t *triggers.Trigger) error {
	panic("implement me")
}

// RemoveBuyTrigger removes a users buy trigger for the corresponding stock
func (u RedisDatabase) RemoveBuyTrigger(user string, stock string) (*triggers.Trigger, error) {
	panic("implement me")
}

// GetBuyTrigger gets a user's trigger for the specified stock, if one exists
func (u RedisDatabase) GetBuyTrigger(user string, stock string) (*triggers.Trigger, error) {
	panic("implement me")
}

// GetStock returns the users available balance of said stock
func (u RedisDatabase) GetStock(user string, stock string) (int, error) {
	conn := u.getConn()
	resp, err := redis.Int(conn.Do("HGET", user+":Stocks", stock))
	return resp, err
}

// RemoveStock removes int stocks from the users account
func (u RedisDatabase) RemoveStock(user string, stock string, amount int) error {
	conn := u.getConn()
	_, err := conn.Do("HINCRBY", user+":Stocks", stock, -amount)
	return err
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
	conn := u.getConn()
	_, err := conn.Do("INCRBYFLOAT", user+":Balance", amount)
	if err != nil {
		panic(err)
	}
	conn.Close()
	return nil
}

// GetFunds returns the amount of available funds in a users account
func (u RedisDatabase) GetFunds(user string) (decimal.Decimal, error) {
	conn := u.getConn()
	r, err := redis.String(conn.Do("GET", user+":Balance"))
	if err != nil {
		panic(err)
	}
	conn.Close()
	receivedValue, err := decimal.NewFromString(r)
	return receivedValue, err
}

// RemoveFunds remove n funds from the user's account
func (u RedisDatabase) RemoveFunds(user string, amount decimal.Decimal) error {
	conn := u.getConn()
	_, err := conn.Do("INCRBYFLOAT", user+":Balance", amount.Neg())
	if err != nil {
		panic(err)
	}
	conn.Close()
	return nil
}

// AddStock adds shares to the user account
func (u RedisDatabase) AddStock(user string, stock string, shares int) error {
	conn := u.getConn()
	_, err := conn.Do("HSET", user+":Stocks", stock, shares)
	return err
}

// DeleteKey deletes a key in the database
// use this function with caution...
func (u RedisDatabase) DeleteKey(key string) {
	conn := u.getConn()
	conn.Do("DEL", key)
	conn.Close()
}
