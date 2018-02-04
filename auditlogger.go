package transactionserver

import (
	"net/http"
	"log"
	"strconv"
	"github.com/shopspring/decimal"
)

type Logger interface {
	QuoteServer(string, int, *QuoteReply) error
	AccountTransaction(server string, transNum int, action string, user interface{}, funds interface{}) error
	SystemError(server string, transNum int, command string, user interface{}, stock interface{}, filename interface{},
	funds interface{}, errorMsg interface{}) error
}

type AuditLogger struct {
	addr string
}

func (al AuditLogger) SystemError(server string, transNum int, command string, user interface{}, stock interface{}, filename interface{},
	funds interface{}, errorMsg interface{}) error{
	params := map[string]string {
		"server": server,
		"transactionNum": strconv.Itoa(transNum),
		"command": command,
	}
	if user != nil {
		params["username"] = user.(string)
	}
	if stock != nil {
		params["stocksymbol"] = stock.(string)
	}
	if filename != nil {
		params["filename"] = filename.(string)
	}
	if funds != nil {
		params["funds"] = funds.(decimal.Decimal).String()
	}
	if errorMsg != nil {
		params["errormessage"] = errorMsg.(string)
	}
	return al.SendLog("/errorEvent", params)
}

func (al AuditLogger) AccountTransaction(server string, transactionNum int, action string, user interface{}, funds interface{}) error {
	params := map[string]string {
		"server":  server,
		"transactionNum": strconv.Itoa(transactionNum),
		"action": action,
	}
	if user != nil {
		params["username"] = user.(string)
	}
	if funds !=  nil {
		params["funds"] = funds.(decimal.Decimal).String()
	}
	return al.SendLog("/accountTransaction", params)
}


func (al AuditLogger) QuoteServer(server string, transactionNum int, rep *QuoteReply) error {
	params := map[string]string {
		"server":  server,
		"transactionNum": strconv.Itoa(transactionNum),
		"price": rep.quote.String(),
		"stockSymbol": rep.stock,
		"username": rep.user,
		"quoteServerTime": strconv.FormatUint(rep.time, 10),
		"cryptoKey": rep.key,
	}
	return al.SendLog("/quoteServer", params)
}

func (al AuditLogger) SendLog(slash string, params map[string]string) error {
	req, err := http.NewRequest("GET", al.addr + slash, nil)
	if err != nil {
		log.Print(err)
	}

	url := req.URL.Query()
	for k, v := range params {
		url.Add(k, v)
	}

	req.URL.RawQuery = url.Encode()
	client := &http.Client{}
	_, err = client.Do(req)
	return err
}