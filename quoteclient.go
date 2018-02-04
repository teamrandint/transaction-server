package transactionserver

import (
	"github.com/patrickmn/go-cache"
	"github.com/shopspring/decimal"
	"regexp"
	"time"
	"bufio"
	"net"
	"fmt"
	"strconv"
)

type QuoteClientI interface {
	Query(string, string, int) (decimal.Decimal, error)
}

type QuoteClient struct {
	name    string
	addr    string
	cache   *cache.Cache
	re      *regexp.Regexp
	logger  Logger
}


type QuoteReply struct {
	quote decimal.Decimal
	stock string
	user  string
	time  uint64
	key   string
}

func NewQuoteClient(logger Logger) *QuoteClient {
	re := regexp.MustCompile("(?P<quote>.+),(?P<stock>.+),(?P<user>.+),(?P<time>.+),(?P<key>.+)")
	return &QuoteClient{
		name:    "quoteserve",
		addr:    "quoteserve.seng:4444",
		cache:   cache.New(time.Minute, time.Minute),
		re:      re,
		logger: logger,
	}
}

func (q *QuoteClient) Query(u string, s string, transNum int) (decimal.Decimal, error) {
	quote, found := q.cache.Get(s)
	if found {
		d, _ := decimal.NewFromString(quote.(string))
		return d, nil
	}
	conn, err := net.Dial("tcp", q.addr)
	if  err != nil {
		return decimal.Decimal{}, err
	}
	request := fmt.Sprintf("%s,%s\n", s, u)
	fmt.Fprintf(conn, request)
	message, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return decimal.Decimal{}, err
	}
	reply := q.getReply(message)
	err = q.logger.QuoteServer(q.name, transNum, reply)
	if err != nil {
		fmt.Print("Error logging to the audit server")
	}
	q.cache.Set(reply.stock, reply.quote, cache.DefaultExpiration)
	return reply.quote, nil
}

func (q *QuoteClient) getReply(msg string) *QuoteReply {
	n1 := q.re.SubexpNames()
	r2 := q.re.FindAllStringSubmatch(msg, -1)[0]

	res := map[string]string{}
	for i, n := range r2 {
		res[n1[i]] = n
	}

	quote, _ := decimal.NewFromString(res["quote"])
	timestamp, _ := strconv.ParseUint(res["time"], 10, 64)
	return &QuoteReply{
		quote: quote,
		stock: res["stock"],
		user:  res["user"],
		time:  timestamp,
		key:   res["key"],
	}
}