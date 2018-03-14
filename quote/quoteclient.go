package quoteclient

import (
	"github.com/shopspring/decimal"
	"os"
	"net/http"
	"log"
	"fmt"
	"strconv"
	"io/ioutil"
)

var addr = os.Getenv("quoteaddr")
var port = os.Getenv("quoteport")

func Query(user string, stock string, transNum int) (decimal.Decimal, error) {
	req, err := http.NewRequest("GET", addr + ":" + port + "/quote", nil)
	if err != nil {
		log.Print(err)
	}
	q := req.URL.Query()
	q.Add("user", user)
	q.Add("stock", stock)
	q.Add("transNum", strconv.Itoa(transNum))
	req.URL.RawQuery = q.Encode()

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error connecting to the quote server: %s", err.Error())
		return decimal.Decimal{}, err
	}
	amount, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading body: %s", err.Error())
		return decimal.Decimal{}, err
	}
	return decimal.NewFromString(string(amount))
}
