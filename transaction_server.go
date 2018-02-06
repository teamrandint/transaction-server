package transactionserver

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type TransactionServer struct {
	Name         string
	Addr         string
	Server       Server
	Logger       Logger
	UserDatabase UserDatabase
	QuoteClient  QuoteClientI
}

func NewTransactionServer(serverAddr string, databaseAddr string, auditAddr string) *TransactionServer {
	server := NewSocketServer(serverAddr)

	database := &RedisDatabase{
		addr: databaseAddr,
	}

	logger := AuditLogger{
		addr: auditAddr,
	}

	quoteClient := NewQuoteClient(logger)

	ts := &TransactionServer{
		Name:         "transactionserve",
		Addr:         serverAddr,
		Server:       server,
		Logger:       logger,
		UserDatabase: database,
		QuoteClient:  quoteClient,
	}

	server.route("ADD,<user>,<amount>", ts.Add)
	server.route("QUOTE,<user>,<stock>", ts.Quote)
	server.route("BUY,<user>,<stock>,<amount>", ts.Buy)
	server.route("COMMIT_BUY,<user>", ts.CommitBuy)
	server.route("CANCEL_BUY,<user>", ts.CancelBuy)
	server.route("SELL,<user>,<stock>,<amount>", ts.Sell)
	server.route("COMMIT_SELL,<user>", ts.CommitBuy)
	server.route("CANCEL_SELL,<user>", ts.CancelBuy)
	server.route("SET_BUY_AMOUNT,<user>,<stock>,<amount>", ts.SetBuyAmount)
	server.route("CANCEL_SET_BUY,<user>,<stock>", ts.CancelSetBuy)
	server.route("SET_BUY_TRIGGER,<user>,<stock>,<amount>", ts.SetBuyTrigger)
	server.route("SET_SELL_AMOUNT,<user>,<stock>,<amount>", ts.SetSellAmount)
	server.route("CANCEL_SET_SELL,<user>,<stock>", ts.CancelSetSell)
	server.route("DUMPLOG,<user>,<filename>", ts.DumpLogUser)
	server.route("DUMPLOG,<filename>", ts.DumpLog)
	server.route("DISPLAY_SUMMARY,<user>", ts.DisplaySummary)
	return ts
}

// Params: user, amount
// Purpose: Add the given amount of money to the user's account
// PostCondition: the user's account is increased by the amount of money specified
func (ts TransactionServer) Add(params ...string) string {
	user := params[0]
	amount, err := decimal.NewFromString(params[1])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "ADD", user, nil, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	err = ts.UserDatabase.AddFunds(user, amount)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "ADD", user, nil, nil, amount,
			"Failed to add amount to the database for user")
	}
	ts.Logger.AccountTransaction(ts.Name, ts.Server.TransactionNum(), "ADD", user, amount)
	return "1"
}

// Params: user, stock
// Purpose: Get the current quote for the stock for the specified user
// PostCondition: the current price of the specified stock is displayed to the user
func (ts TransactionServer) Quote(params ...string) string {
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
func (ts TransactionServer) Buy(params ...string) string {
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
			fmt.Sprintf("Error connecting to the database to get funds: %s", err.Error()))
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
			fmt.Sprintf("Error connecting to the quote server: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.RemoveFunds(user, cost)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			fmt.Sprintf("Error connecting to the database to remove funds: %s", err.Error()))
		return "-1"
	}
	err = ts.UserDatabase.PushBuy(user, stock, cost, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "BUY", user, stock, nil, amount,
			fmt.Sprintf("Error connecting to the database to push buy command: %s", err.Error()))
		return "-1"
	}

	ts.Logger.AccountTransaction(ts.Name, ts.Server.TransactionNum(), "remove", user, amount)
	return "1"
}

// Params: user
// Purpose:  Commits the most recently executed BUY command
// Pre-Conditions: The user must have executed a BUY command within the previous 60 seconds
// Post-Conditions:
// 		(a) the user's cash account is decreased by the amount user to purchase the stock
// 		(b) the user's account for the given stock is increased by the purchase amount
func (ts TransactionServer) CommitBuy(params ...string) string {
	user := params[0]
	ts.Logger.SystemEvent(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, nil, nil, nil)
	stock, _, shares, err := ts.UserDatabase.PopBuy(user)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddStock(user, stock, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_BUY", user, stock, nil, nil,
			fmt.Sprintf("Error connecting to database to add stock: %s", err.Error()))
		return "-1"
	}
	return "1"
}

// Param: user
// Purpose: Cancels the most recently executed BUY Command
// Pre-Condition: The user must have executed a BUY command within the previous 60 seconds
// Post-Condition: The last BUY command is canceled and any allocated system resources are reset and released.
func (ts TransactionServer) CancelBuy(params ...string) string {
	user := params[0]
	stock, cost, _, err := ts.UserDatabase.PopBuy(user)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_BUY", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddFunds(user, cost)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_BUY", user, stock, nil, nil,
			fmt.Sprintf("Error connecting to database to add funds: %s", err.Error()))
		return "-1"
	}
	return "1"
}

// Param: user, stock, amount
// Purpose: Sell the specified dollar mount of the stock currently held by the specified user at the current price.
// Pre-condition: The user's account for the given stock must be greater than or equal to the amount being sold.
// Post-condition: The user is asked to confirm or cancel the given transaction
func (ts TransactionServer) Sell(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	cost, shares, err := ts.getMaxPurchase(user, stock, amount, nil)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			fmt.Sprintf("Could not connect to the quote server: %s", err.Error()))
		return "-1"
	}
	curr, err := ts.UserDatabase.GetStock(user, stock)
	if curr < shares {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			"Cannot sell more stock than you own")
		return "-1"
	}

	err = ts.UserDatabase.RemoveStock(user, stock, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			fmt.Sprintf("Error removing stock from database: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.PushSell(user, stock, cost, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SELL", user, stock, nil, amount,
			fmt.Sprintf("Error pushing sell command to database: %s", err.Error()))
		return "-1"
	}
	return "-1"
}

// Params: user
// Purpose: Commits the most recently executed SELL command
// Pre-Conditions: The user must have executed a SELL command within the previous 60 seconds
// Post-Conditions:
// 		(a) the user's account for the given stock is decremented by the sale amount
// 		(b) the user's cash account is increased by the sell amount
func (ts TransactionServer) CommitSell(params ...string) string {
	user := params[0]
	ts.Logger.SystemEvent(ts.Name, ts.Server.TransactionNum(), "COMMIT_SELL", user, nil, nil, nil)
	stock, cost, _, err := ts.UserDatabase.PopSell(user)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_SELL", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddFunds(user, cost)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "COMMIT_SELL", user, stock, nil, cost,
			fmt.Sprintf("Error connecting to database to add funds: %s", err.Error()))
		return "-1"
	}
	return "1"

}

// Params: user
// Purpose: Cancels the most recently executed SELL Command
// Pre-conditions: The user must have executed a SELL command within the previous 60 seconds
// Post-conditions: The last SELL command is canceled and any allocated system resources are reset and released.
func (ts TransactionServer) CancelSell(params ...string) string {
	user := params[0]
	stock, _, shares, err := ts.UserDatabase.PopSell(user)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SELL", user, nil, nil, nil,
			fmt.Sprintf("Error connecting to database to pop command: %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddStock(user, stock, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SELL", user, stock, nil, nil,
			fmt.Sprintf("Error connecting to database to add stock: %s", err.Error()))
		return "-1"
	}
	return "1"
}

// Params: user, stock, amount
// Purpose: Sets a defined amount of the given stock to buy when the current stock price is less than or equal to the BUY_TRIGGER
// Pre-condition: The user's cash account must be greater than or equal to the BUY amount at the time the transaction occurs
// Post-condition:
// 		(a) a reserve account is created for the BUY transaction to hold the specified amount in reserve for when the transaction is triggered
// 		(b) the user's cash account is decremented by the specified amount
// 		(c) when the trigger point is reached the user's stock account is updated to reflect the BUY transaction.
func (ts TransactionServer) SetBuyAmount(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}

	curr, err := ts.UserDatabase.GetFunds(user)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Could not get funds from database: %s", err.Error()))
		return "-1"
	}
	if curr.LessThan(amount) {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, amount,
			"Not enough funds to execute command")
		return "-1"
	}

	err = ts.UserDatabase.RemoveFunds(user, amount)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Error removing funds from database:  %s", err.Error()))
		return "-1"
	}

	trig := NewBuyTrigger(user, stock, ts.QuoteClient, amount, ts.buyExecute)
	err = ts.UserDatabase.AddBuyTrigger(user, stock, trig)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Error adding buy trigger to database:  %s", err.Error()))
		return "-1"
	}
	return "1"
}

// Params: user, stock
// Purpose: Cancels a SET_BUY command issued for the given stock
// Pre-condition: The must have been a SET_BUY Command issued for the given stock by the user
// Post-condition:
// 		(a) All accounts are reset to the values they would have had had the SET_BUY Command not been issued
// 		(b) the BUY_TRIGGER for the given user and stock is also canceled.
func (ts TransactionServer) CancelSetBuy(params ...string) string {
	user := params[0]
	stock := params[1]
	trigger, err := ts.UserDatabase.RemoveBuyTrigger(user, stock)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SET_BUY", user, stock, nil, nil,
			fmt.Sprintf("Error getting buy trigger:  %s", err.Error()))
		return "-1"
	}

	err = ts.UserDatabase.AddFunds(user, trigger.BuySellAmount)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SET_BUY", user, stock, nil, nil,
			fmt.Sprintf("Error adding funds:  %s", err.Error()))
		return "-1"
	}

	trigger.Cancel()
	return "1"
}

// Params: user, stock, amount
// Purpose: Sets the trigger point base on the current stock price when any SET_BUY will execute.
// Pre-conditions: The user must have specified a SET_BUY_AMOUNT prior to setting a SET_BUY_TRIGGER
// Post-conditions: The set of the user's buy triggers is updated to include the specified trigger
func (ts TransactionServer) SetBuyTrigger(params ...string) string {
	user := params[0]
	stock := params[1]
	triggerAmount, err := decimal.NewFromString(params[2])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_TRIGGER", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}
	trig, err := ts.UserDatabase.GetBuyTrigger(user, stock)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_BUY_TRIGGER", user, stock, nil, triggerAmount,
			fmt.Sprintf("Could not get buy trigger from database: %s", err.Error()))
		return "-1"
	}
	trig.Start(triggerAmount, ts.Server.TransactionNum())
	return "1"
}

// Params: user, stock, amount
// Purpose: Sets a defined amount of the specified stock to sell when the current stock price is equal or greater than the sell trigger point
// Pre-conditions: The user must have the specified amount of stock in their account for that stock.
// Post-conditions: A trigger is initialized for this username/stock symbol combination, but is not complete until SET_SELL_TRIGGER is executed.
func (ts TransactionServer) SetSellAmount(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}

	_, shares, err := ts.getMaxPurchase(user, stock, amount, nil)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Could not connect to quote server: %s", err.Error()))
		return "-1"
	}

	curr, err := ts.UserDatabase.GetStock(user, stock)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Could not get stock from database: %s", err.Error()))
		return "-1"
	}

	if shares > curr {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, amount,
			"Cannot set sell trigger for more stock than you own")
		return "-1"
	}

	trig := NewSellTrigger(user, stock, ts.QuoteClient, amount, ts.sellExecute)
	err = ts.UserDatabase.AddSellTrigger(user, stock, trig)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_AMOUNT", user, stock, nil, amount,
			fmt.Sprintf("Could not add sell trigger to database: %s", err.Error()))
		return "-1"
	}

	return "1"
}

// Params: user, stock, amount
// Purpose: Sets the stock price trigger point for executing any SET_SELL triggers associated with the given stock and user
// Pre-Conditions: The user must have specified a SET_SELL_AMOUNT prior to setting a SET_SELL_TRIGGER
// Post-Conditions:
// 		(a) a reserve account is created for the specified amount of the given stock
// 		(b) the user account for the given stock is reduced by the max number of stocks that could be purchased and
// 		(c) the set of the user's sell triggers is updated to include the specified trigger.
func (ts TransactionServer) SetSellTrigger(params ...string) string {
	user := params[0]
	stock := params[1]
	amount, err := decimal.NewFromString(params[2])
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, nil,
			"Could not parse add amount to decimal")
		return "-1"
	}

	trig, err := ts.UserDatabase.GetSellTrigger(user, stock)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, amount,
			fmt.Sprintf("Could not get sell trigger to database: %s", err.Error()))
		return "-1"
	}

	_, shares, err := ts.getMaxPurchase(user, stock, amount, nil)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, amount,
			fmt.Sprintf("Could not connect to quote server: %s", err.Error()))
		return "-1"
	}
	err = ts.UserDatabase.RemoveStock(user, stock, shares)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, amount,
			fmt.Sprintf("Could not remove stock from database: %s", err.Error()))
		return "-1"
	}

	trig.Start(amount, ts.Server.TransactionNum())
	ts.Logger.SystemEvent(ts.Name, ts.Server.TransactionNum(), "SET_SELL_TRIGGER", user, stock, nil, amount)
	return "1"

}

// Params: user, stock
// Purpose: Cancels the SET_SELL associated with the given stock and user
// Pre-Conditions: The user must have had a previously set SET_SELL for the given stock
// Post-Conditions:
// 		(a) The set of the user's sell triggers is updated to remove the sell trigger associated with the specified stock
// 		(b) all user account information is reset to the values they would have been if the given SET_SELL command had not been issued
func (ts TransactionServer) CancelSetSell(params ...string) string {
	user := params[0]
	stock := params[1]
	trigger, err := ts.UserDatabase.RemoveSellTrigger(user, stock)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SET_SELL", user, stock, nil, nil,
			fmt.Sprintf("Error getting buy trigger:  %s", err.Error()))
		return "-1"
	}

	_, reserved, _ := ts.getMaxPurchase(user, stock, trigger.BuySellAmount, trigger.TriggerAmount)
	err = ts.UserDatabase.AddStock(user, stock, reserved)
	if err != nil {
		ts.Logger.SystemError(ts.Name, ts.Server.TransactionNum(), "CANCEL_SET_SELL", user, stock, nil, nil,
			fmt.Sprintf("Error adding stock to database:  %s", err.Error()))
		return "-1"
	}

	trigger.Cancel()
	return "1"
}

// Params: user, filename
// Purpose: Print out the history of the users transactions to the user specified file
// Post-Condition: The history of the user's transaction are written to the specified file.
func (ts TransactionServer) DumpLogUser(params ...string) string {
	user := params[0]
	filename := params[1]
	ts.Logger.DumpLog(filename, user)
	return "1"
}

// Params: filename
// Purpose: Print out to the specified file the complete set of transactions that have occurred in the system.
// Pre-Condition: Can only be executed from the supervisor (root/administrator) account.
// Post-Condition: Places a complete log file of all transactions that have occurred in the system into the file specified by filename
func (ts TransactionServer) DumpLog(params ...string) string {
	filename := params[0]
	ts.Logger.DumpLog(filename, nil)
	return "1"
}

// Params: user
// Purpose: Provides a summary to the client of the given user's transaction history and the current status of their accounts as well as any set buy or sell triggers and their parameters
// Post-Condition: A summary of the given user's transaction history and the current status of their accounts as well as any set buy or sell triggers and their parameters is displayed to the user.
func (ts TransactionServer) DisplaySummary(params ...string) string {
	panic("not implemented")
}

func (ts TransactionServer) sellExecute(trigger *Trigger) {
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

func (ts TransactionServer) buyExecute(trigger *Trigger) {
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
