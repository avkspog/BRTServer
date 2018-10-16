package tcp

import (
	"bufio"
	"fmt"
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
		c.server.leavingCh <- c
		c.Conn.Close()
	}()

	scanner := bufio.NewScanner(c.Conn)
	for {
		scanned := scanner.Scan()
		if !scanned {
			if err := scanner.Err(); err != nil {
				fmt.Printf("%v", err)
				return
			}
			break
		}
		b := scanner.Bytes()
		c.server.messagesCh <- &receiveMessage{client: c, data: b}
	}
}

func (c *Client) Close() {
	c.Conn.Close()
}
