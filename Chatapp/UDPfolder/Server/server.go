package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	clients      = make(map[string]*net.UDPAddr)
	lastActivity = make(map[string]time.Time)
	mu           sync.Mutex
)

func main() {
	addr, _ := net.ResolveUDPAddr("udp", ":6000")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()
	fmt.Println("Server listening on port 6000")

	// Set up logging to file
	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	go cleanInactiveClients()

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Println("Error reading from client:", err)
			continue
		}

		msg := strings.TrimSpace(string(buffer[:n]))
		log.Printf("Received from %s: %s\n", clientAddr.String(), msg)
		fmt.Printf("Received from %s: %s\n", clientAddr.String(), msg)

		if msg == "/shutdown" {
			log.Println("Shutdown command received. Server is shutting down.")
			fmt.Println("Shutdown command received. Server is shutting down.")
			return
		}

		mu.Lock()
		clients[clientAddr.String()] = clientAddr
		lastActivity[clientAddr.String()] = time.Now()
		mu.Unlock()

		// Ignore heartbeat messages
		if msg == "/ping" {
			continue
		}

		// Broadcast message to other clients
		mu.Lock()
		for _, addr := range clients {
			if addr.String() != clientAddr.String() {
				conn.WriteToUDP([]byte(fmt.Sprintf("%s says: %s", clientAddr.String(), msg)), addr)
			}
		}
		mu.Unlock()
	}
}

func cleanInactiveClients() {
	for {
		time.Sleep(10 * time.Second)
		mu.Lock()
		now := time.Now()
		for id, last := range lastActivity {
			if now.Sub(last) > 30*time.Second {
				log.Printf("Removing inactive client: %s\n", id)
				delete(clients, id)
				delete(lastActivity, id)
			}
		}
		mu.Unlock()
	}
}
