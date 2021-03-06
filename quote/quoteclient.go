package quoteclient

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"seng468/transaction-server/logger"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/shopspring/decimal"
)

type QuoteClientI interface {
	Query(string, string, int) (decimal.Decimal, error)
}

type QuoteClient struct {
	name   string
	addr   string
	cache  *cache.Cache
	re     *regexp.Regexp
	logger logger.Logger
}

type QuoteReply struct {
	quote decimal.Decimal
	stock string
	user  string
	time  uint64
	key   string
}

func NewQuoteClient(logger logger.Logger) *QuoteClient {
	re := regexp.MustCompile("(?P<quote>.+),(?P<stock>.+),(?P<user>.+),(?P<time>.+),(?P<key>.+)")
	return &QuoteClient{
		name:   "quoteserve",
		addr:   "quoteserve.seng:4444",
		cache:  cache.New(time.Minute, time.Minute),
		re:     re,
		logger: logger,
	}
}

func (q *QuoteClient) Query(u string, s string, transNum int) (decimal.Decimal, error) {
	quote, found := q.cache.Get(s)
	if found {
		d, _ := decimal.NewFromString(quote.(string))
		return d, nil
	}
	conn, err := net.DialTimeout("tcp", q.addr, 30*time.Millisecond)
	if err != nil {
		return decimal.Decimal{}, err
	}
	request := fmt.Sprintf("%s,%s\n", s, u)
	fmt.Fprintf(conn, request)
	message, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return decimal.Decimal{}, err
	}
	reply := q.getReply(message)
	go q.logger.QuoteServer(q.name, transNum, reply.quote.String(), reply.stock,
		reply.user, reply.time, reply.key)
	q.cache.Set(reply.stock, reply.quote.String(), cache.DefaultExpiration)
	conn.Close()
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
