// Copyright 2014 Bowery, Inc.
package pubsub

import (
	"log"
	"net"
)

var clients []net.Conn

func Run() {
	// Create TCP listener on port 3002.
	listener, err := net.Listen("tcp", ":3002")
	if err != nil {
		log.Println(err)
	}
	defer listener.Close()

	// Accept new connections.
	for {
		conn, _ := listener.Accept()
		log.Println("Client Connected")
		clients = append(clients, conn)
	}
}

func Publish(data []byte) error {
	for _, c := range clients {
		_, err := c.Write(data)
		if err != nil {
			return err
		}
	}
	return nil
}
