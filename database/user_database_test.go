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
	db.DeleteKey("AAA")
}

func TestGetUserInfo(t *testing.T) {
	db := RedisDatabase{"tcp", ":6379"}
	_, error := db.GetUserInfo("AAA")
	if error != nil {
		t.Error(error)
	}
}

func TestRemoveFunds(t *testing.T) {
	db := RedisDatabase{"tcp", ":6379"}
	dollar, err := decimal.NewFromString("23.01")
	err2 := db.AddFunds("F", dollar)
	if err != nil || err2 != nil {
		t.Error(err, err2)
	}
	err = db.RemoveFunds("F", dollar)
	zero, _ := db.GetFunds("F")

	if zero.String() != "0" {
		t.Error("Account should be 0")
	}
	db.DeleteKey("F:Balance")
}

func TestGetFunds(t *testing.T) {
	db := RedisDatabase{"tcp", ":6379"}
	dollar, err := decimal.NewFromString("23.01")

	err2 := db.AddFunds("fundGetter", dollar)
	amount, err2 := db.GetFunds("fundGetter")

	if err != nil || err2 != nil {
		t.Error(err, err2)
	}

	if amount.String() != dollar.String() {
		t.Error("Amounts not equal, 23.01,", amount)
	}
	db.DeleteKey("fundGetter:Balance")
}
