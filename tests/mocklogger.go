package tests

import (
	"seng468/transaction-server"
)

type MockLogger struct {

}

func (MockLogger) AccountTransaction(server string, transNum int, action string, user interface{}, funds interface{}) error {
	return nil
}

func (MockLogger) SystemError(server string, transNum int, command string, user interface{}, stock interface{}, filename interface{},
	funds interface{}, errorMsg interface{}) error {
	return nil
}

func (MockLogger) QuoteServer(string, int, *transactionserver.QuoteReply) error {
	return nil
}

