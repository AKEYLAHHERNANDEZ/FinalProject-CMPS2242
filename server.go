package main

import (
	"fmt"
	"net"
	"sync"
)

var clients = make(map[string]*net.UDPAddr)
var mu sync.Mutex

func main() {
	addr, _ := net.ResolveUDPAddr("udp", ":6000")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()
	fmt.Println("Server listening on port 6000")

	buffer := make([]byte, 1024)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from client:", err)
			continue
		}

		msg := string(buffer[:n])
		fmt.Printf("Received from %s: %s\n", clientAddr.String(), msg)

		mu.Lock()
		clients[clientAddr.String()] = clientAddr
		mu.Unlock()

		// Broadcast to all clients
		mu.Lock()
		for _, addr := range clients {
			if addr.String() != clientAddr.String() { // don't echo back
				conn.WriteToUDP([]byte(fmt.Sprintf("%s says: %s", clientAddr.String(), msg)), addr)
			}
		}
		mu.Unlock()
	}
}
