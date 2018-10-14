package tcp

import (
	"bufio"
	"net"
	"sync"
)

type Client struct {
	Conn      net.Conn
	waitGroup *sync.WaitGroup
	closeCh   chan struct{}
}

type receiveMessage struct {
	client *Client
	data   *[]byte
}

func NewClient(conn net.Conn, wg *sync.WaitGroup) *Client {
	client := &Client{
		Conn:      conn,
		waitGroup: wg,
		closeCh:   make(chan struct{}, 1),
	}
	return client
}

func (c *Client) Listen() {
	defer func() {
		c.Conn.Close()
		c.waitGroup.Done()
	}()

	reader := bufio.NewReader(c.Conn)

	type receiveData struct {
		data *[]byte
		err  error
	}

	for {
		receiveDataCh := make(chan receiveData, 1)

		go func() {
			data, err := reader.ReadBytes('\r')
			receiveDataCh <- receiveData{&data, err}
		}()

		select {
		case m := <-receiveDataCh:
			if m.err != nil {
				leaving <- c
				return
			}
			messages <- &receiveMessage{client: c, data: m.data}
		case <-c.closeCh:
			return
		}
	}
}

func (c *Client) Close() {
	c.closeCh <- struct{}{}
}
