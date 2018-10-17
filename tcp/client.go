package tcp

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"
)

type Client struct {
	Conn net.Conn

	idleTimeout time.Duration
	server      *Server
	closeCh     chan struct{}
}

type receiveMessage struct {
	client *Client
	data   []byte
}

func NewClient(conn net.Conn, srv *Server) *Client {
	client := &Client{
		Conn:        conn,
		server:      srv,
		idleTimeout: srv.IdleTimeout,
		closeCh:     make(chan struct{}, 1),
	}
	return client
}

func (c *Client) Listen() {
	defer func() {
		c.Conn.Close()
		c.server.waitGroup.Done()
		c.server.leavingCh <- c
		fmt.Println("debug: Client.Listen() gorutine closed")
	}()

	c.updateDeadline()

	timeout := time.After(c.idleTimeout)
	scrCh := make(chan bool)
	scanner := bufio.NewScanner(c)

	for {
		go func(scanCh chan bool) {
			result := scanner.Scan()
			if !result {
				c.closeCh <- struct{}{}
			} else {
				scanCh <- result
			}
		}(scrCh)

		select {
		case scanned := <-scrCh:
			if !scanned {
				if err := scanner.Err(); err != nil {
					fmt.Printf("%v\n", err)
					return
				}
				break
			}
			b := scanner.Bytes()
			c.server.messagesCh <- &receiveMessage{client: c, data: b}
			timeout = time.After(c.idleTimeout)

		case <-timeout:
			log.Printf("timeout: %v\n", c.Conn.RemoteAddr())
			return

		case <-c.closeCh:
			return
		}
	}
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

func (c *Client) updateDeadline() {
	idleDeadline := time.Now().Add(c.idleTimeout)
	c.Conn.SetDeadline(idleDeadline)
}
