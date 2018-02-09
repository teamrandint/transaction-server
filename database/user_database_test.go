package database

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestAddUser(t *testing.T) {
	db := RedisDatabase{"tcp", ":6379"}
	_, err := db.GetUserInfo("AAA")
	if err != nil {
		t.Error(err)
	}
}

func TestAddFunds(t *testing.T) {
	db := RedisDatabase{"tcp", ":6379"}
	dollar, err := decimal.NewFromString("23.01")
	err2 := db.AddFunds("AAA", dollar)
	if err != nil || err2 != nil {
		t.Error(err, err2)
	}
}
