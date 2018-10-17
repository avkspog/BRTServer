package tcp

import (
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	IdleTimeout time.Duration

	address    string
	listener   *net.TCPListener
	waitGroup  *sync.WaitGroup
	mu         *sync.Mutex
	clients    map[*Client]struct{}
	signalCh   chan os.Signal
	enteringCh chan *Client
	leavingCh  chan *Client
	messagesCh chan *receiveMessage
	shutdownCh chan struct{}
	*event
}

type event struct {
	onServerStarted  func(addr *net.TCPAddr)
	onServerStopped  func()
	onNewConnection  func(c *Client)
	onConnectionLost func(c *Client)
	onMessageReceive func(c *Client, data *[]byte)
}

func NewServer(address string) *Server {
	event := &event{
		onServerStarted:  func(addr *net.TCPAddr) {},
		onServerStopped:  func() {},
		onNewConnection:  func(c *Client) {},
		onConnectionLost: func(c *Client) {},
		onMessageReceive: func(c *Client, data *[]byte) {},
	}

	server := &Server{
		IdleTimeout: 10 * time.Minute,

		address:    address,
		waitGroup:  &sync.WaitGroup{},
		mu:         &sync.Mutex{},
		clients:    make(map[*Client]struct{}),
		signalCh:   make(chan os.Signal),
		enteringCh: make(chan *Client),
		leavingCh:  make(chan *Client),
		messagesCh: make(chan *receiveMessage),
		shutdownCh: make(chan struct{}),
		event:      event,
	}
	return server
}

func (s *Server) Listen() error {
	addr, _ := net.ResolveTCPAddr("tcp", s.address)
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	defer func() {
		listener.Close()
		s.shutdownCh <- struct{}{}
		s.onServerStopped()
	}()

	s.broadcaster()

	signal.Notify(s.signalCh, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	go s.onServerStarted(addr)

	type accepted struct {
		conn net.Conn
		err  error
	}

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
			client := NewClient(accept.conn, s)
			s.enteringCh <- client

		case <-s.signalCh:
			log.Println("shutting down server...")
			s.listener.Close()
			s.closeConnections()
			s.waitGroup.Wait()
			return nil
		}
	}
}

func (s *Server) broadcaster() {
	go func() {
		for {
			select {
			case client := <-s.enteringCh:
				s.waitGroup.Add(1)
				go client.Listen()

				s.addClient(client)
				go s.onNewConnection(client)

			case client := <-s.leavingCh:
				s.removeClient(client)
				go s.onConnectionLost(client)

			case msg := <-s.messagesCh:
				go s.onMessageReceive(msg.client, &msg.data)

			case <-s.shutdownCh:
				return
			}
		}
	}()
}

func (s *Server) Shutdown() {
	s.signalCh <- syscall.SIGINT
}

func (s *Server) closeConnections() {
	defer s.mu.Unlock()
	s.mu.Lock()
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

func (s *Server) Clients() map[*Client]struct{} {
	return s.clients
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
