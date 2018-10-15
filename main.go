package main

import (
	"BRTServer/tcp"
	"fmt"
	"log"
	"net"
	"os"
)

var server *tcp.Server

func main() {
	host := "localhost"
	port := "5326"

	if len(os.Args[1:]) == 2 {
		host = os.Args[1]
		port = os.Args[2]
	}
	server = tcp.NewServer(host + ":" + port)

	server.OnServerStarted(func(addr *net.TCPAddr) {
		log.Printf("KRTS server started on address: %v", addr.String())
	})

	server.OnServerStopped(func() {
		log.Println("KRTS server stopped")
	})

	server.OnNewConnection(func(c *tcp.Client) {
		log.Printf("accepted connection from: %v", c.Conn.RemoteAddr())
	})

	server.OnMessageReceive(func(c *tcp.Client, data *[]byte) {
		n := len(*data)
		fmt.Println("message len = ", n)
		if n > 0 {
			message := make([]byte, n)
			copy(message, *data)
			mm := string(message)
			log.Printf("%v message: %v", c.Conn.RemoteAddr(), mm)
		}
	})

	server.OnConnectionLost(func(c *tcp.Client) {
		log.Printf("closing connection from %v", c.Conn.RemoteAddr())
	})

	if err := server.Listen(); err != nil {
		log.Printf("Fatal error: %s", err.Error())
		os.Exit(1)
	}
}
