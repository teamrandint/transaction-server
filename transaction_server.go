package main

import (
	"fmt"
	"seng468/transaction-server/database"
	"seng468/transaction-server/logger"
	"seng468/transaction-server/quote"
	"seng468/transaction-server/socketserver"
	"seng468/transaction-server/trigger"
	"github.com/shopspring/decimal"
)

// TransactionServer holds the main components of the module itself
type TransactionServer struct {
	Name            string
	Addr            string
	Server          socketserver.Server
	Logger          logger.Logger
	UserDatabase    database.UserDatabase
	QuoteClient     quoteclient.QuoteClientI
	BuyTriggers map[string]*triggers.Trigger
	SellTriggers map[string]*triggers.Trigger
}

func main() {
	serverAddr := "localhost:8888"
	databaseAddr := "localhost:6379"
	auditAddr := "http://localhost:8080"

	server := socketserver.NewSocketServer(serverAddr)
	database := &database.RedisDatabase{Addr: databaseAddr}
	logger := logger.AuditLogger{Addr: auditAddr}
	buyTriggers := make(map[string]*triggers.Trigger)
	sellTriggers := make(map[string]*triggers.Trigger)
	quoteClient := quoteclient.NewQuoteClient(logger)

	ts := &TransactionServer{
		Name:            "transactionserve",
		Addr:            serverAddr,
		Server:          server,
		Logger:          logger,
		UserDatabase:    database,
		QuoteClient:     quoteClient,
		BuyTriggers:	 buyTriggers,
		SellTriggers:    sellTriggers,
	}

	server.Route("ADD,<user>,<amount>", ts.Add)
	server.Route("QUOTE,<user>,<stock>", ts.Quote)
	server.Route("BUY,<user>,<stock>,<amount>", ts.Buy)
	server.Route("COMMIT_BUY,<user>", ts.CommitBuy)
	server.Route("CANCEL_BUY,<user>", ts.CancelBuy)
	server.Route("SELL,<user>,<stock>,<amount>", ts.Sell)
	server.Route("COMMIT_SELL,<user>", ts.CommitBuy)
	server.Route("CANCEL_SELL,<user>", ts.CancelBuy)
	server.Route("SET_BUY_AMOUNT,<user>,<stock>,<amount>", ts.SetBuyAmount)
	server.Route("CANCEL_SET_BUY,<user>,<stock>", ts.CancelSetBuy)
	server.Route("SET_BUY_TRIGGER,<user>,<stock>,<amount>", ts.SetBuyTrigger)
	server.Route("SET_SELL_AMOUNT,<user>,<stock>,<amount>", ts.SetSellAmount)
	server.Route("CANCEL_SET_SELL,<user>,<stock>", ts.CancelSetSell)
	server.Route("DUMPLOG,<user>,<filename>", ts.DumpLogUser)
	server.Route("DUMPLOG,<filename>", ts.DumpLog)
	server.Route("DISPLAY_SUMMARY,<user>", ts.DisplaySummary)
	server.Run()
}

// Add the given amount of money to the user's account
// Params: user, amount
// PostCondition: the user's account is increased by the amount of money specified
func (ts TransactionServer) Add(params ...string) string {
	user := params[0]
	amount, err := decimal.NewFromString(params[1])
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "ADD", user, nil, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	err = ts.UserDatabase.AddFunds(user, amount)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "ADD", user, nil, nil, amount,
			"Failed to add amount to the database for user")
	}
	ts.Logger.AccountTransaction(ts.Name, ts.Server.TransactionNum(), "ADD", user, amount)
	return "1"
}

// Quote gets the current quote for the stock for the specified user
// Params: user, stock
// PostCondition: the current price of the specified stock is displayed to the user
func (ts TransactionServer) Quote(params ...string) string {
	user := params[0]
	stock := params[1]
	dec, err := ts.QuoteClient.Query(user, stock, ts.Server.TransactionNum())
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "QUOTE", user, stock, nil, nil,
			err.Error())
		return "-1"
	}
	return dec.StringFixed(2)
}

// Buy the dollar amount of the stock for the specified user at the current price.
// Params: user, stock, amount
// PreCondition: The user's account must be greater or equal to the amount of the purchase.
// PostCondition: The user is asked to confirm or cancel the transaction
func (ts TransactionServer) Buy(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	curr, err := ts.UserDatabase.GetFunds(user)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			fmt.Sprintf("Error connecting to the database to get funds: %s", err.Error()))
		return "-1"
	}
	if curr.LessThan(amount) {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			"Not enough funds to issue buy order")
		return "-1"
	}

	cost, shares, err := ts.getMaxPurchase(user, stock, amount, nil)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			fmt.Sprintf("Error connecting to the quote server: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.RemoveFunds(user, cost)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			fmt.Sprintf("Error connecting to the database to remove funds: %s", err.Error()))
		return "-1"
	}
	err = ts.UserDatabase.PushBuy(user, stock, cost, shares)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			fmt.Sprintf("Error connecting to the database to push buy command: %s", err.Error()))
		return "-1"
	}

	go ts.Logger.AccountTransaction(ts.Name, ts.Server.TransactionNum(), "remove", user, amount)
	return "1"
}

// CommitBuy commits the most recently executed BUY command
// Params: user
// Pre-Conditions: The user must have executed a BUY command within the previous 60 seconds
// Post-Conditions:
// 		(a) the user's cash account is decreased by the amount user to purchase the stock
// 		(b) the user's account for the given stock is increased by the purchase amount
func (ts TransactionServer) CommitBuy(params ...string) string {
	user := params[0]
	go ts.Logger.SystemEvent(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, nil, nil, nil)
	stock, _, shares, err := ts.UserDatabase.PopBuy(user)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddStock(user, stock, shares)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, stock, nil, nil,
			fmt.Sprintf("Error connecting to database to add stock: %s", err.Error()))
		return "-1"
	}
	return "1"
}

// CancelBuy cancels the most recently executed BUY Command
// Param: user
// Pre-Condition: The user must have executed a BUY command within the previous 60 seconds
// Post-Condition: The last BUY command is canceled and any allocated system resources are reset and released.
func (ts TransactionServer) CancelBuy(params ...string) string {
	user := params[0]
	stock, cost, _, err := ts.UserDatabase.PopBuy(user)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_BUY", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddFunds(user, cost)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_BUY", user, stock, nil, nil,
			fmt.Sprintf("Error connecting to database to add funds: %s", err.Error()))
		return "-1"
	}
	return "1"
}

// Sell the specified dollar mount of the stock currently held by the specified
// user at the current price.
// Param: user, stock, amount
// Pre-condition: The user's account for the given stock must be greater than
// 		or equal to the amount being sold.
// Post-condition: The user is asked to confirm or cancel the given transaction
func (ts TransactionServer) Sell(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	cost, shares, err := ts.getMaxPurchase(user, stock, amount, nil)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			fmt.Sprintf("Could not connect to the quote server: %s", err.Error()))
		return "-1"
	}
	curr, err := ts.UserDatabase.GetStock(user, stock)
	if curr < shares {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			"Cannot sell more stock than you own")
		return "-1"
	}

	err = ts.UserDatabase.RemoveStock(user, stock, shares)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			fmt.Sprintf("Error removing stock from database: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.PushSell(user, stock, cost, shares)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			fmt.Sprintf("Error pushing sell command to database: %s", err.Error()))
		return "-1"
	}
	return "-1"
}

// CommitSell commits the most recently executed SELL command
// Params: user
// Pre-Conditions: The user must have executed a SELL command within the previous 60 seconds
// Post-Conditions:
// 		(a) the user's account for the given stock is decremented by the sale amount
// 		(b) the user's cash account is increased by the sell amount
func (ts TransactionServer) CommitSell(params ...string) string {
	user := params[0]
	go ts.Logger.SystemEvent(ts.Name, ts.Server.TransactionNum(), "COMMIT_SELL", user, nil, nil, nil)
	stock, cost, _, err := ts.UserDatabase.PopSell(user)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_SELL", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddFunds(user, cost)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_SELL", user, stock, nil, cost,
			fmt.Sprintf("Error connecting to database to add funds: %s", err.Error()))
		return "-1"
	}
	return "1"

}

// CancelSell cancels the most recently executed SELL Command
// Params: user
// Pre-conditions: The user must have executed a SELL command within the previous 60 seconds
// Post-conditions: The last SELL command is canceled and any allocated system resources are reset and released.
func (ts TransactionServer) CancelSell(params ...string) string {
	user := params[0]
	stock, _, shares, err := ts.UserDatabase.PopSell(user)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SELL", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddStock(user, stock, shares)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SELL", user, stock, nil, nil,
			fmt.Sprintf("Error connecting to database to add stock: %s", err.Error()))
		return "-1"
	}
	return "1"
}

// SetBuyAmount sets a defined amount of the given stock to buy when the
// current stock price is less than or equal to the BUY_TRIGGER
// Params: user, stock, amount
// Pre-condition: The user's cash account must be greater than or equal to the
//		BUY amount at the time the transaction occurs
// Post-condition:
// 		(a) a reserve account is created for the BUY transaction to hold the
//			specified amount in reserve for when the transaction is triggered
// 		(b) the user's cash account is decremented by the specified amount
// 		(c) when the trigger point is reached the user's stock account is
//			updated to reflect the BUY transaction.
func (ts TransactionServer) SetBuyAmount(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}

	curr, err := ts.UserDatabase.GetFunds(user)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Could not get funds from database: %s", err.Error()))
		return "-1"
	}

	if curr.LessThan(amount) {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, amount,
			"Not enough funds to execute command")
		return "-1"
	}

	// TODO move funds to reserve account
	err = ts.UserDatabase.RemoveFunds(user, amount)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Error removing funds from database:  %s", err.Error()))
		return "-1"
	}

	trig := triggers.NewBuyTrigger(user, stock, ts.QuoteClient, amount, ts.buyExecute)
	ts.BuyTriggers[user+","+stock] = trig
	return "1"
}

// CancelSetBuy cancels a SET_BUY command issued for the given stock
// Params: user, stock
// The must have been a SET_BUY Command issued for the given stock by the user
// Post-condition:
// 		(a) All accounts are reset to the values they would have had had the
//			SET_BUY Command not been issued
// 		(b) the BUY_TRIGGER for the given user and stock is also canceled.
func (ts TransactionServer) CancelSetBuy(params ...string) string {
	user := params[0]
	stock := params[1]

	//TODO add funds back from reserve account

	trigger := ts.getBuyTrigger(user, stock)
	if trigger == nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SET_BUY", user, stock, nil, nil,
			"No existing buy trigger for this user and stock")
		return "-1"
	}
	trigger.Cancel()
	return "1"
}

// SetBuyTrigger sets the trigger point base on the current stock price when
// any SET_BUY will execute.
// Params: user, stock, amount
// Pre-conditions: The user must have specified a SET_BUY_AMOUNT prior to
//		 setting a SET_BUY_TRIGGER
// Post-conditions: The set of the user's buy triggers is updated to
//		include the specified trigger
func (ts TransactionServer) SetBuyTrigger(params ...string) string {
	user := params[0]
	stock := params[1]
	triggerAmount, err := decimal.NewFromString(params[2])
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_TRIGGER", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	trig := ts.getBuyTrigger(user, stock)
	if trig == nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_TRIGGER", user, stock, nil, nil,
			"No existing buy trigger for this user and stock")
		return "-1"
	}
	trig.Start(triggerAmount, ts.Server.TransactionNum())
	return "1"
}

// SetSellAmount sets a defined amount of the specified stock to sell when
// the current stock price is equal or greater than the sell trigger point
// Params: user, stock, amount
// Pre-conditions: The user must have the specified amount of stock in their
//		account for that stock.
// Post-conditions: A trigger is initialized for this username/stock symbol
//		combination, but is not complete until SET_SELL_TRIGGER is executed.
func (ts TransactionServer) SetSellAmount(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}

	_, shares, err := ts.getMaxPurchase(user, stock, amount, nil)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Could not connect to quote server: %s", err.Error()))
		return "-1"
	}

	curr, err := ts.UserDatabase.GetStock(user, stock)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Could not get stock from database: %s", err.Error()))
		return "-1"
	}

	if shares > curr {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, amount,
			"Cannot set sell trigger for more stock than you own")
		return "-1"
	}

	trig := triggers.NewSellTrigger(user, stock, ts.QuoteClient, amount, ts.sellExecute)
	ts.SellTriggers[user+","+stock] = trig
	return "1"
}

// SetSellTrigger sets the stock price trigger point for executing any
// SET_SELL triggers associated with the given stock and user
// Params: user, stock, amount
// Pre-Conditions: The user must have specified a SET_SELL_AMOUNT prior to
//		setting a SET_SELL_TRIGGER
// Post-Conditions:
// 		(a) a reserve account is created for the specified amount of the
//			given stock
// 		(b) the user account for the given stock is reduced by the max number
//			of stocks that could be purchased and
// 		(c) the set of the user's sell triggers is updated to include the
//			specified trigger.
func (ts TransactionServer) SetSellTrigger(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}

	trig := ts.getSellTrigger(user, stock)
	if trig == nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, nil,
			"No existing sell trigger for this user and stock")
		return "-1"
	}

	_, shares, err := ts.getMaxPurchase(user, stock, trig.BuySellAmount, amount)
	// TODO add shares to reserve account

	err = ts.UserDatabase.RemoveStock(user, stock, shares)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, amount,
			fmt.Sprintf("Could not remove stock from database: %s", err.Error()))
		return "-1"
	}

	trig.Start(amount, ts.Server.TransactionNum())
	go ts.Logger.SystemEvent(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, amount)
	return "1"

}

// CancelSetSell cancels the SET_SELL associated with the given stock and user
// Pre-Conditions: The user must have had a previously set SET_SELL for the given stock
// Post-Conditions:
// 		(a) The set of the user's sell triggers is updated to remove the sell trigger associated with the specified stock
// 		(b) all user account information is reset to the values they would have been if the given SET_SELL command had not been issued
func (ts TransactionServer) CancelSetSell(params ...string) string {
	user := params[0]
	stock := params[1]
	trigger := ts.getSellTrigger(user, stock)
	if trigger == nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SELL_TRIGGER", user, stock, nil, nil,
			"No existing sell trigger for this user and stock")
		return "-1"
	}

	// TODO add from reserve account
	_, reserved, _ := ts.getMaxPurchase(user, stock, trigger.BuySellAmount, trigger.TriggerAmount)
	err := ts.UserDatabase.AddStock(user, stock, reserved)
	if err != nil {
		go ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SET_SELL", user, stock, nil, nil,
			fmt.Sprintf("Error adding stock to database:  %s", err.Error()))
		return "-1"
	}

	trigger.Cancel()
	return "1"
}

// DumpLogUser Print out the history of the users transactions
// to the user specified file
func (ts TransactionServer) DumpLogUser(params ...string) string {
	filename := params[0]
	user := params[1]
	go ts.Logger.DumpLog(filename, user)
	return "1"
}

// DumpLog prints out to the specified file the complete set of transactions
// that have occurred in the system.
// Can only be executed from the supervisor (root/administrator) account.
func (ts TransactionServer) DumpLog(params ...string) string {
	filename := params[0]
	go ts.Logger.DumpLog(filename, nil)
	return "1"
}

// DisplaySummary provides a summary to the client of the given user's
// transaction history and the current status of their accounts as well
// as any set buy or sell triggers and their parameters.
func (ts TransactionServer) DisplaySummary(params ...string) string {
	panic("not implemented")
}

// getBuyTrigger returns a pointer to the running buy trigger that corresponds
// to the given user, stock combo.
// If there is not a matching running trigger, returns nil
func (ts TransactionServer) getBuyTrigger(user string, stock string) *triggers.Trigger {
	if val, ok := ts.BuyTriggers[user+","+stock]; ok {
		return val
	}
	return nil
}

// getSellTrigger returns a pointer to the running sell trigger that corresponds
// to the given user, stock combo.
// If there is not a matching running trigger, returns nil
func (ts TransactionServer) getSellTrigger(user string, stock string) *triggers.Trigger {
	if val, ok := ts.SellTriggers[user+","+stock]; ok {
		return val
	}
	return nil
}

func (ts TransactionServer) sellExecute(trigger *triggers.Trigger) {
	cost, shares, err := ts.getMaxPurchase(trigger.User, trigger.Stock, trigger.BuySellAmount, nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	_, reserved, _ := ts.getMaxPurchase(trigger.User, trigger.Stock, trigger.BuySellAmount, trigger.TriggerAmount)
	ts.UserDatabase.AddFunds(trigger.User, cost)
	ts.UserDatabase.AddStock(trigger.User, trigger.Stock, reserved-shares)
	ts.UserDatabase.RemoveBuyTrigger(trigger.User, trigger.Stock)
}

func (ts TransactionServer) buyExecute(trigger *triggers.Trigger) {
	cost, shares, err := ts.getMaxPurchase(trigger.User, trigger.Stock, trigger.BuySellAmount, nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	ts.UserDatabase.AddFunds(trigger.User, trigger.BuySellAmount.Sub(cost))
	ts.UserDatabase.AddStock(trigger.User, trigger.Stock, shares)
	ts.UserDatabase.RemoveBuyTrigger(trigger.User, trigger.Stock)
}

func (ts TransactionServer) getMaxPurchase(user string, stock string, amount decimal.Decimal, stockPrice interface{}) (money decimal.Decimal, shares int, err error) {
	dec, err := ts.QuoteClient.Query(user, stock, ts.Server.TransactionNum())
	if err != nil {
		return decimal.Decimal{}, 0, err
	}
	sharesDec := amount.Div(dec).Floor()
	sharesF, _ := sharesDec.Float64()
	shares = int(sharesF)
	money = dec.Mul(sharesDec)
	return money.Round(2), shares, nil
}
