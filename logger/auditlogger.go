package logger

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/shopspring/decimal"
)

type Logger interface {
	QuoteServer(server string, transNum int,
		price string, stock string, user string, qsTime uint64, key string)

	AccountTransaction(server string, transNum int,
		action string, user interface{}, funds interface{})

	SystemError(server string, transNum int,
		command string, user interface{}, stock interface{},
		filename interface{},
		funds interface{}, errorMsg interface{})

	SystemEvent(server string, transNum int,
		command string, username interface{}, stock interface{},
		filename interface{}, funds interface{})

	DumpLog(filename string, username interface{})
}

type AuditLogger struct {
	Addr string
}

func (al AuditLogger) DumpLog(filename string, username interface{}) {
	params := map[string]string{
		"filename": filename,
	}
	if username != nil {
		params["username"] = username.(string)
	}
	al.SendLog("/dumpLog", params)
}

func (al AuditLogger) SystemEvent(server string, transNum int, command string, username interface{}, stock interface{},
	filename interface{}, funds interface{}) {
	params := map[string]string{
		"server":         server,
		"transactionNum": strconv.Itoa(transNum),
		"command":        command,
	}
	if username != nil {
		params["username"] = username.(string)
	}
	if stock != nil {
		params["stockSymbol"] = stock.(string)
	}
	if filename != nil {
		params["filename"] = filename.(string)
	}
	if funds != nil {
		params["funds"] = funds.(decimal.Decimal).String()
	}
	al.SendLog("/systemEvent", params)
}

func (al AuditLogger) SystemError(server string, transNum int, command string, user interface{}, stock interface{}, filename interface{},
	funds interface{}, errorMsg interface{}) {
	params := map[string]string{
		"server":         server,
		"transactionNum": strconv.Itoa(transNum),
		"command":        command,
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
	al.SendLog("/errorEvent", params)
}

func (al AuditLogger) AccountTransaction(server string, transactionNum int, action string, user interface{}, funds interface{}) {
	params := map[string]string{
		"server":         server,
		"transactionNum": strconv.Itoa(transactionNum),
		"action":         action,
	}
	if user != nil {
		params["username"] = user.(string)
	}
	if funds != nil {
		params["funds"] = funds.(decimal.Decimal).String()
	}
	al.SendLog("/accountTransaction", params)
}

func (al AuditLogger) QuoteServer(server string, transactionNum int,
	price string, stock string, user string, qsTime uint64, key string) {
	params := map[string]string{
		"server":          server,
		"transactionNum":  strconv.Itoa(transactionNum),
		"price":           price,
		"stockSymbol":     stock,
		"username":        user,
		"quoteServerTime": strconv.FormatUint(qsTime, 10),
		"cryptoKey":       key,
	}
	al.SendLog("/quoteServer", params)
}

func (al AuditLogger) SendLog(slash string, params map[string]string) {
	req, err := http.NewRequest("GET", al.Addr+slash, nil)
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
	if err != nil {
		fmt.Printf("Error connecting to the audit server for  %s command:  %s", slash, err.Error())
	}
	defer req.Body.Close()
}
