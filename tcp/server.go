package tcp

import (
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Server struct {
	address   string
	listener  *net.TCPListener
	waitGroup *sync.WaitGroup
	signalCh  chan os.Signal
	mu        *sync.Mutex
	clients   map[*Client]struct{}
	*event
}

type event struct {
	onServerStarted  func(addr *net.TCPAddr)
	onServerStopped  func()
	onNewConnection  func(c *Client)
	onConnectionLost func(c *Client)
	onMessageReceive func(c *Client, data *[]byte)
}

var (
	entering = make(chan *Client)
	leaving  = make(chan *Client)
	messages = make(chan *receiveMessage)
)

func NewServer(address string) *Server {
	event := &event{
		onServerStarted:  func(addr *net.TCPAddr) {},
		onServerStopped:  func() {},
		onNewConnection:  func(c *Client) {},
		onConnectionLost: func(c *Client) {},
		onMessageReceive: func(c *Client, data *[]byte) {},
	}

	server := &Server{
		address:   address,
		waitGroup: &sync.WaitGroup{},
		signalCh:  make(chan os.Signal),
		mu:        &sync.Mutex{},
		clients:   make(map[*Client]struct{}),
		event:     event,
	}

	return server
}

func (s *Server) Listen() error {
	addr, _ := net.ResolveTCPAddr("tcp", s.address)
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	go s.broadcaster()

	defer func() {
		listener.Close()
		s.onServerStopped()
	}()

	signal.Notify(s.signalCh, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	go s.onServerStarted(addr)

	type accepted struct {
		conn net.Conn
		err  error
	}

	for {
		c := make(chan accepted, 1)
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

			client := NewClient(accept.conn, s.waitGroup)
			entering <- client
			s.waitGroup.Add(1)
			go client.Listen()

		case <-s.signalCh:
			log.Println("shutting down server...")
			s.listener.Close()
			go s.closeConnections()
			s.waitGroup.Wait()
			return nil
		}
	}
}

func (s *Server) broadcaster() {
	for {
		select {
		case client := <-entering:
			s.addClient(client)
			go s.onNewConnection(client)
		case client := <-leaving:
			s.removeClient(client)
			go s.onConnectionLost(client)
		case msg := <-messages:
			go s.onMessageReceive(msg.client, msg.data)
		}
	}
}

func (s *Server) StopGraceful() {
	s.signalCh <- syscall.SIGINT
}

func (s *Server) closeConnections() {
	for c := range s.clients {
		if c != nil {
			c.Close()
		}
	}
}

func (s *Server) addClient(c *Client) {
	defer s.mu.Unlock()
	s.mu.Lock()
	s.clients[c] = struct{}{}
}

func (s *Server) removeClient(c *Client) {
	defer s.mu.Unlock()
	s.mu.Lock()
	delete(s.clients, c)
}

func (s *Server) Clients() *map[*Client]struct{} {
	return &s.clients
}

func (s *Server) OnServerStopped(callback func()) {
	s.onServerStopped = callback
}

func (s *Server) OnServerStarted(callback func(addr *net.TCPAddr)) {
	s.onServerStarted = callback
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
