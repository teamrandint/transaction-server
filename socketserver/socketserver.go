package socketserver

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"regexp"
)

type Server interface {
	TransactionNum() int
}

type SocketServer struct {
	addr     string
	routeMap map[string]func(args ...string) string
	transNum int
}

func (s SocketServer) TransactionNum() int {
	return s.transNum
}

func NewSocketServer(addr string) SocketServer {
	return SocketServer{
		addr:     addr,
		routeMap: make(map[string]func(args ...string) string),
		transNum: 0,
	}
}

func getParamsFromRegex(regex string, msg string) []string {
	re, _ := regexp.Compile(regex)
	match := re.FindAllStringSubmatch(msg, -1)[0]
	var params []string
	for _, m := range match {
		m = string(bytes.Trim([]byte(m), "\x00"))
		params = append(params, m)
	}
	return params[1:]
}

func (s SocketServer) buildRoutePattern(pattern string) string {
	re := regexp.MustCompile(`(<\w+>)`)
	return re.ReplaceAllString(pattern, `(.+)`) // `(?P\1.+)`
}

func (s SocketServer) Route(pattern string, f func(args ...string) string) {
	regex := s.buildRoutePattern(pattern)
	s.routeMap[regex] = f
}

func (s SocketServer) Run() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer l.Close()
	fmt.Println("Listening on " + s.addr)
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			continue
		}
		s.transNum++
		go s.handleRequest(conn)
	}
}

func (s SocketServer) getRoute(command string) (func(args ...string) string, []string) {
	for regex, function := range s.routeMap {
		re, err := regexp.Compile(regex)
		if err != nil {
			fmt.Printf(regex)
			panic(err)
		}
		if re.MatchString(command) {
			return function, getParamsFromRegex(regex, command)
		}

	}
	return nil, nil
}

// Handles incoming requests.
func (s SocketServer) handleRequest(conn net.Conn) {
	// Make a buffer to hold incoming data.
	buf := make([]byte, 1024)
	// Read the incoming connection into the buffer.
	_, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error reading:", err.Error())
		return
	}
	function, params := s.getRoute(string(buf[:]))
	if function == nil {
		fmt.Printf("Error: command not implemented '%s'\n", string(buf[:]))
		return
	}
	fmt.Println(string(buf[:]))
	res := function(params...)
	// Send a response back to person contacting us.
	conn.Write([]byte(res))
	// Close the connection when you're done with it.
	conn.Close()
}
