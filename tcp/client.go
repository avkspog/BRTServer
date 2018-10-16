package tcp

import (
	"bufio"
	"log"
	"net"
)

type Client struct {
	Conn    net.Conn
	server  *Server
	closeCh chan struct{}
}

type receiveMessage struct {
	client *Client
	data   []byte
}

func NewClient(conn net.Conn, srv *Server) *Client {
	client := &Client{
		Conn:    conn,
		server:  srv,
		closeCh: make(chan struct{}, 1),
	}
	return client
}

func (c *Client) Listen() {
	defer func() {
		c.server.waitGroup.Done()
		c.server.leaving <- c
		c.Conn.Close()
	}()

	scanner := bufio.NewScanner(c.Conn)
	for {
		scanned := scanner.Scan()
		if !scanned {
			if err := scanner.Err(); err != nil {
				log.Println(err)
				return
			}
			break
		}
		c.server.messages <- &receiveMessage{client: c, data: scanner.Bytes()}
	}
}

func (c *Client) Close() {
	c.Conn.Close()
}
