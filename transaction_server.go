package transactionserver

import (
	"github.com/shopspring/decimal"
	"fmt"
)

type TransactionServer struct {
	Name string
	Addr string
	Server Server
	Logger Logger
	UserDatabase UserDatabase
	QuoteClient QuoteClientI
}

func NewTransactionServer(serverAddr string, databaseAddr string, auditAddr string) *TransactionServer {
	server := NewSocketServer(serverAddr)

	database := &RedisDatabase {
		addr: databaseAddr,
	}

	logger := AuditLogger {
		addr: auditAddr,
	}

	quoteClient := NewQuoteClient(logger)

	ts := &TransactionServer{
		Name: "transactionserve",
		Addr: serverAddr,
		Server: server,
		Logger: logger,
		UserDatabase: database,
		QuoteClient: quoteClient,
	}

	server.route("ADD,<user>,<amount>", ts.Add)
	server.route("QUOTE,<user>,<stock>", ts.Quote)
	server.route("BUY,<user>,<stock>,<amount>", ts.Buy)
	return ts
}

// Params: user, amount
// Purpose: Add the given amount of money to the user's account
// PostCondition: the user's account is increased by the amount of money specified
func (ts  TransactionServer) Add(params...  string) string {
	user := params[0]
	amount, err := decimal.NewFromString(params[1])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "ADD", user, nil, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	err = ts.UserDatabase.AddFunds(user, amount)
	if err !=  nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "ADD", user, nil, nil, amount,
			"Failed to add amount to the database for user")
	}
	err = ts.Logger.AccountTransaction(ts.Name, ts.Server.TransactionNum(), "ADD", user, amount)
	if err != nil {
		fmt.Print(err)
	}
	return "1"
}

// Params: user, stock
// Purpose: Get the current quote for the stock for the specified user
// PostCondition: the current price of the specified stock is displayed to the user
func (ts TransactionServer) Quote(params... string) string {
	user := params[0]
	stock := params[1]
	dec, err := ts.QuoteClient.Query(user, stock, ts.Server.TransactionNum())
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "QUOTE", user, stock, nil, nil,
			err.Error())
		return "-1"
	}
	return dec.StringFixed(2)
}

// Params: user, stock, amount
// Purpose: Buy the dollar amount of the stock for the specified user at the current price.
// PreCondition: The user's account must be greater or equal to the amount of the purchase.
// PostCondition: The user is asked to confirm or cancel the transaction
func (ts TransactionServer) Buy(params... string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	curr, err := ts.UserDatabase.GetFunds(user)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			err.Error())
		return "-1"
	}
	if curr.LessThan(amount) {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			"Not enough funds to issue buy order")
		return "-1"
	}

	cost, shares, err := ts.getMaxPurchase(user, stock, amount, nil)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			err.Error())
		return "-1"
	}

	err = ts.UserDatabase.RemoveFunds(user, cost)
	if  err != nil  {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			err.Error())
		return "-1"
	}
	err = ts.UserDatabase.PushBuy(user, stock, cost, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			err.Error())
		return "-1"
	}

	ts.Logger.AccountTransaction(ts.Name, ts.Server.TransactionNum(), "BUY", user, amount)
	if err != nil {
		fmt.Print(err)
	}
	return "1"
}

// Params: user
// Purpose:  Commits the most recently executed BUY command
// Pre-Conditions: The user must have executed a BUY command within the previous 60 seconds
// Post-Conditions:
// 		(a) the user's cash account is decreased by the amount user to purchase the stock
// 		(b) the user's account for the given stock is increased by the purchase amount
func (ts TransactionServer) CommitBuy(params... string) string {
	user := params[0]
	//System event
	stock, _, shares, err := ts.UserDatabase.PopBuy(user)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, stock, nil, nil,
			err.Error())
		return "-1"
	}

	err = ts.UserDatabase.AddStock(user, stock, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, stock, nil, nil,
			err.Error())
		return "-1"
	}
	return "1"
}

func (ts TransactionServer) getMaxPurchase(user string, stock string, amount decimal.Decimal, stockPrice interface{}) (money decimal.Decimal, shares int, err error) {
	stockDec := decimal.Decimal{}
	if stockPrice == nil {
		dec, err := ts.QuoteClient.Query(user, stock, ts.Server.TransactionNum())
		if err != nil {
			return decimal.Decimal{}, 0, err
		}
		stockDec = dec
	}
	stockDec = stockPrice.(decimal.Decimal)
	sharesDec := amount.Div(stockDec).Floor()
	sharesF, _ := sharesDec.Float64()
	shares = int(sharesF)
	money = stockDec.Mul(sharesDec)
	return money.Round(2), shares, nil
}



