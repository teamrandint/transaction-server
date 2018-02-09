package database

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

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

	AddReserveFunds(string, decimal.Decimal) error
	GetReserveFunds(string) (decimal.Decimal, error)
	RemoveReserveFunds(string, decimal.Decimal) error

	AddReserveStock(user string, stock string, shares int) error
	GetReserveStock(user string, stock string) (int, error)
	RemoveReserveStock(user string, stock string, amount int) error

	PushBuy(user string, stock string, cost decimal.Decimal, shares int) error
	PopBuy(user string) (stock string, cost decimal.Decimal, shares int, err error)
	PushSell(user string, stock string, cost decimal.Decimal, shares int) error
	PopSell(user string) (stock string, cost decimal.Decimal, shares int, err error)
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

// PushSell adds a record of the users requested sell to their account
func (u RedisDatabase) PushSell(user string, stock string, cost decimal.Decimal, shares int) error {
	return u.pushOrder("Sell", user, stock, cost, shares)
}

// PopSell removes a users most recent requested sell
func (u RedisDatabase) PopSell(user string) (stock string, cost decimal.Decimal, shares int, err error) {
	return u.popOrder("Sell", user)
}

// PushBuy adds a record of the users requested buy to their account
func (u RedisDatabase) PushBuy(user string, stock string, cost decimal.Decimal, shares int) error {
	// Expires in 60s
	return u.pushOrder("Buy", user, stock, cost, shares)
}

// PopBuy removes a users most recent requested buy
func (u RedisDatabase) PopBuy(user string) (stock string, cost decimal.Decimal, shares int, err error) {
	return u.popOrder("Buy", user)
}

func (u RedisDatabase) pushOrder(transType string, user string,
	stock string, cost decimal.Decimal, shares int) error {
	accountSuffix := ""
	if transType == "Buy" {
		accountSuffix = ":BuyOrders"
	} else if transType == "Sell" {
		accountSuffix = ":SellOrders"
	} else {
		return errors.New("Bad transaction type of " + transType)
	}

	encoded := u.encodeOrder(stock, cost, shares)

	conn := u.getConn()
	_, err := redis.Int64(conn.Do("RPUSH", user+accountSuffix, encoded))
	conn.Close()
	return err
}

func (u RedisDatabase) popOrder(transType string, user string) (stock string, cost decimal.Decimal, shares int, err error) {
	accountSuffix := ""
	if transType == "Buy" {
		accountSuffix = ":BuyOrders"
	} else if transType == "Sell" {
		accountSuffix = ":SellOrders"
	} else {
		return stock, cost, shares, errors.New("Bad transaction type of " + transType)
	}

	conn := u.getConn()
	recv, err := redis.String(conn.Do("RPOP", user+accountSuffix))
	conn.Close()

	stock, cost, shares = u.decodeOrder(recv)
	return stock, cost, shares, err
}

// Encodes a buy or sell order into a string, to be pushed onto the pending orders stack
// Returns a string following the format of:
//		"stock:cost:shares"
func (u RedisDatabase) encodeOrder(stock string, cost decimal.Decimal, shares int) string {
	return stock + ":" + cost.String() + ":" + strconv.Itoa(shares)
}

// Performs the opposite of encodeOrder
func (u RedisDatabase) decodeOrder(order string) (stock string, cost decimal.Decimal, shares int) {
	split := strings.Split(order, ":")
	stock = split[0]
	cost, _ = decimal.NewFromString(split[1])
	shares, _ = strconv.Atoi(split[2])

	return stock, cost, shares
}

// AddFunds adds amount dollars to the user account
func (u RedisDatabase) AddFunds(user string, amount decimal.Decimal) error {
	_, err := u.fundAction("Add", user, ":Balance", amount)
	return err
}

// GetFunds returns the amount of available funds in a users account
func (u RedisDatabase) GetFunds(user string) (decimal.Decimal, error) {
	amount := decimal.NewFromFloat(0.0)
	return u.fundAction("Get", user, ":Balance", amount)
}

// RemoveFunds remove n funds from the user's account
// amount is the absolute value of the funds being removed
func (u RedisDatabase) RemoveFunds(user string, amount decimal.Decimal) error {
	_, err := u.fundAction("Remove", user, ":Balance", amount)
	return err
}

// AddReserveFunds adds funds to a user's reserve account
func (u RedisDatabase) AddReserveFunds(user string, amount decimal.Decimal) error {
	_, err := u.fundAction("Add", user, ":BalanceReserve", amount)
	return err
}

// GetReserveFunds returns the amount of funds present in a users reserve account
func (u RedisDatabase) GetReserveFunds(user string) (decimal.Decimal, error) {
	amount := decimal.NewFromFloat(0.0)
	return u.fundAction("Get", user, ":BalanceReserve", amount)
}

// RemoveReserveFunds removes n funds from a users account
// Pass in the absoloute value of funds to be removed.
func (u RedisDatabase) RemoveReserveFunds(user string, amount decimal.Decimal) error {
	_, err := u.fundAction("Add", user, ":BalanceReserve", amount)
	return err
}

// stockAction handles the generic stock commands
func (u RedisDatabase) fundAction(action string, user string,
	accountSuffix string, amount decimal.Decimal) (decimal.Decimal, error) {
	command := ""
	if action == "Add" {
		command = "INCRBYFLOAT"
	} else if action == "Get" {
		command = "GET"
	} else if action == "Remove" {
		command = "INCRBYFLOAT"
		amount = amount.Neg()
	} else {
		return decimal.NewFromFloat(0.0), errors.New("Bad action attempt on funds")
	}

	conn := u.getConn()
	var r float64
	var err error
	if action != "Get" {
		r, err = redis.Float64(conn.Do(command, user+accountSuffix, amount))
	} else {
		r, err = redis.Float64(conn.Do(command, user+accountSuffix))

	}
	conn.Close()
	return decimal.NewFromFloat(r), err
}

// GetStock returns the users available balance of said stock
func (u RedisDatabase) GetStock(user string, stock string) (int, error) {
	return u.stockAction("Get", user, ":Stocks", stock, 0)
}

// RemoveStock removes int stocks from the users account
// Send the absolute value of the stock being removed
func (u RedisDatabase) RemoveStock(user string, stock string, amount int) error {
	_, err := u.stockAction("Remove", user, ":Stocks", stock, amount)
	return err
}

// AddStock adds shares to the user account
func (u RedisDatabase) AddStock(user string, stock string, shares int) error {
	_, err := u.stockAction("Add", user, ":Stocks", stock, shares)
	return err
}

// AddReserveStock adds n shares of stock to a user's account
func (u RedisDatabase) AddReserveStock(user string, stock string, amount int) error {
	_, err := u.stockAction("Add", user, ":StocksReserve", stock, amount)
	return err
}

// GetReserveStock returns the amount of shares present in a user's reserve account
func (u RedisDatabase) GetReserveStock(user string, stock string) (int, error) {
	return u.stockAction("Get", user, ":StocksReserve", stock, 0)
}

// RemoveReserveStock removes n shares of stock from a user's reserve account
func (u RedisDatabase) RemoveReserveStock(user string, stock string, amount int) error {
	_, err := u.stockAction("Remove", user, ":StocksReserve", stock, amount)
	return err
}

// stockAction handles the generic stock commands
func (u RedisDatabase) stockAction(action string, user string,
	accountSuffix string, stock string, amount int) (int, error) {
	command := ""
	if action == "Add" {
		command = "HINCRBY"
	} else if action == "Get" {
		command = "HGET"
	} else if action == "Remove" {
		command = "HINCRBY"
		amount = -amount
	} else {
		return 0, errors.New("Bad action attempt on stocks")
	}

	conn := u.getConn()
	var r int
	var err error
	if action != "Get" {
		r, err = redis.Int(conn.Do(command, user+accountSuffix, stock, amount))
	} else {
		r, err = redis.Int(conn.Do(command, user+accountSuffix, stock))

	}
	conn.Close()
	return r, err
}

// DeleteKey deletes a key in the database
// use this function with caution...
func (u RedisDatabase) DeleteKey(key string) {
	conn := u.getConn()
	conn.Do("DEL", key)
	conn.Close()
}
