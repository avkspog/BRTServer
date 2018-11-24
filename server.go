package brts

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	defaultTimeout      time.Duration = 10 * time.Minute
	defaultMessageDelim byte          = '\r'
)

type Server struct {
	idleTimeout  time.Duration
	address      string
	waitGroup    *sync.WaitGroup
	mu           *sync.Mutex
	clients      map[*Client]struct{}
	signalCh     chan os.Signal
	messageDelim byte

	onServerStarted  func(addr *net.TCPAddr)
	onServerStopped  func()
	onNewConnection  func(c *Client)
	onConnectionLost func(c *Client)
	onMessageReceive func(c *Client, data *[]byte)
}

type Client struct {
	Conn net.Conn

	idleTimeout time.Duration
	closeCh     chan struct{}
}

type accepted struct {
	conn net.Conn
	err  error
}

func Create(address string) *Server {
	server := &Server{
		idleTimeout:  defaultTimeout,
		address:      address,
		waitGroup:    &sync.WaitGroup{},
		mu:           &sync.Mutex{},
		clients:      make(map[*Client]struct{}),
		signalCh:     make(chan os.Signal),
		messageDelim: defaultMessageDelim,

		onServerStarted:  func(addr *net.TCPAddr) {},
		onServerStopped:  func() {},
		onNewConnection:  func(c *Client) {},
		onConnectionLost: func(c *Client) {},
		onMessageReceive: func(c *Client, data *[]byte) {},
	}
	return server
}

func newClient(conn net.Conn, timeout time.Duration) *Client {
	client := &Client{
		Conn:        conn,
		idleTimeout: timeout,
		closeCh:     make(chan struct{}),
	}
	return client
}

func (s *Server) Start() error {
	addr, _ := net.ResolveTCPAddr("tcp", s.address)
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	go s.onServerStarted(addr)

	defer func() {
		listener.Close()
		s.onServerStopped()
	}()

	signal.Notify(s.signalCh, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	c := make(chan accepted, 1)
	for {
		go func() {
			conn, err := listener.Accept()
			c <- accepted{conn, err}
		}()

		select {
		case accept := <-c:
			if accept.err != nil {
				log.Printf("error accepting connection %v", err)
				continue
			}
			client := newClient(accept.conn, s.idleTimeout)
			s.waitGroup.Add(1)
			go s.listen(client)

		case <-s.signalCh:
			log.Println("shutting down server...")
			listener.Close()
			s.closeConnections()
			s.waitGroup.Wait()
			return nil
		}
	}
}

func (s *Server) listen(c *Client) {
	s.addClient(c)
	s.onNewConnection(c)

	defer func() {
		c.Conn.Close()
		s.waitGroup.Done()
		s.removeClient(c)
		s.onConnectionLost(c)
	}()

	c.updateDeadline()

	type receiveData struct {
		data *[]byte
		err  error
	}

	timeout := time.After(c.idleTimeout)
	scrCh := make(chan receiveData)
	reader := bufio.NewReader(c)

	for {
		go func(scanCh chan receiveData) {
			data, err := reader.ReadBytes(s.messageDelim)
			if err != nil {
				if err == io.EOF {
					c.closeCh <- struct{}{}
					return
				}
				fmt.Printf("Error %s: %v\n", c.Conn.RemoteAddr(), err)
				c.closeCh <- struct{}{}
			} else {
				scanCh <- receiveData{&data, err}
			}
		}(scrCh)

		select {
		case rcv := <-scrCh:
			timeout = time.After(c.idleTimeout)
			s.onMessageReceive(c, rcv.data)

		case <-timeout:
			log.Printf("timeout: %v\n", c.Conn.RemoteAddr())
			return

		case <-c.closeCh:
			return
		}
	}
}

func (c *Client) updateDeadline() {
	idleDeadline := time.Now().Add(c.idleTimeout)
	c.Conn.SetDeadline(idleDeadline)
}

func (s *Server) Shutdown() {
	s.signalCh <- syscall.SIGINT
}

func (s *Server) closeConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for c := range s.clients {
		if c != nil {
			c.Close()
		}
	}
}

func (s *Server) addClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[c] = struct{}{}
}

func (s *Server) removeClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c)
}

func (c *Client) Read(p []byte) (n int, err error) {
	c.updateDeadline()
	n, err = c.Conn.Read(p)
	return
}

func (c *Client) Close() (err error) {
	err = c.Conn.Close()
	return
}

func (s *Server) SetTimeout(timeout time.Duration) {
	s.idleTimeout = timeout
}

func (s *Server) SetMessageDelim(delim byte) {
	s.messageDelim = delim
}

func (s *Server) Clients() map[*Client]struct{} {
	return s.clients
}

func (s *Server) OnServerStarted(callback func(addr *net.TCPAddr)) {
	s.onServerStarted = callback
}

func (s *Server) OnServerStopped(callback func()) {
	s.onServerStopped = callback
}

func (s *Server) OnNewConnection(callback func(c *Client)) {
	s.onNewConnection = callback
}

func (s *Server) OnConnectionLost(callback func(c *Client)) {
	s.onConnectionLost = callback
}

func (s *Server) OnMessageReceive(callback func(c *Client, data *[]byte)) {
	s.onMessageReceive = callback
}
