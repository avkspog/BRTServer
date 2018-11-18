package main

import (
	"brts"
	"log"
	"net"
	"os"
	"time"
)

var server *brts.Server

func main() {
	host := "127.0.0.1"
	port := "8002"

	server = brts.Create(host + ":" + port)
	server.SetTimeout(15 * time.Second)
	server.SetMessageDelim('\n')

	server.OnServerStarted(func(addr *net.TCPAddr) {
		log.Printf("BRTS server started on address: %v", addr.String())
	})

	server.OnServerStopped(func() {
		log.Println("BRTS server stopped")
	})

	server.OnNewConnection(func(c *brts.Client) {
		log.Printf("accepted connection from: %v", c.Conn.RemoteAddr())
	})

	server.OnMessageReceive(func(c *brts.Client, data *[]byte) {
		mm := string(*data)
		log.Printf("%v message: %v", c.Conn.RemoteAddr(), mm)
	})

	server.OnConnectionLost(func(c *brts.Client) {
		log.Printf("closing connection from %v", c.Conn.RemoteAddr())
	})

	if err := server.Start(); err != nil {
		log.Printf("Fatal error: %s", err.Error())
		os.Exit(1)
	}
}
