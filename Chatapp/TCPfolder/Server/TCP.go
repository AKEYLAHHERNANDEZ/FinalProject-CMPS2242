package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// Client struct represents a single connected user
type Client struct {
	username   string
	conn       net.Conn
	isConnected bool
}

// Message struct represents a chat message
type Message struct {
	text   string
	sender *Client
}

// Global variables to manage clients and messages
var (
	clients         = make(map[*Client]bool) // Map of active clients
	clientsMux      sync.Mutex               // Mutex to safely access clients map
	clientJoinChan  = make(chan *Client)     // Channel for new clients
	clientLeaveChan = make(chan *Client)     // Channel for disconnecting clients
	broadcastChan   = make(chan Message)     // Channel for broadcasting messages
)

func main() {
	// Command-line flags for host and port configuration
	host := flag.String("host", "0.0.0.0", "Server host address")
	port := flag.String("port", "4000", "Server port number")
	flag.Parse()

	// Start listening on provided address
	addr := net.JoinHostPort(*host, *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()
	log.Printf("TCP Chat Server running on %s", addr)

	// Graceful shutdown handler (Ctrl+C or kill signal)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")

		// Close channels
		close(clientJoinChan)
		close(clientLeaveChan)
		close(broadcastChan)

		listener.Close() // This causes Accept() to fail, exiting the loop
	}()

	// Start background services for message broadcasting and client management
	go broadcastMessages()
	go manageClients()

	// Accept and handle incoming connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				break // Exit loop on normal shutdown
			}
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConnection(conn) // Handle client in a new goroutine
	}

	// Final cleanup: disconnect all remaining clients
	clientsMux.Lock()
	for client := range clients {
		client.conn.Close()
		delete(clients, client)
	}
	clientsMux.Unlock()
}

// Handles a single client connection
func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Prompt user for their username
	conn.Write([]byte("Enter your username: "))
	reader := bufio.NewReader(conn)
	username, err := reader.ReadString('\n')
	if err != nil {
		log.Printf("Error reading username: %v", err)
		return
	}
	username = username[:len(username)-1] // Remove newline character

	client := &Client{username: username, conn: conn, isConnected: true}

	// Notify server of new client
	clientJoinChan <- client
	defer func() { clientLeaveChan <- client }()

	// Notify all clients that a new user has joined
	broadcastChan <- Message{
		text:   fmt.Sprintf("%s joined the chat", username),
		sender: nil,
	}

	// Listen for messages from the client
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msgText := scanner.Text()

		if msgText == "" {
			continue // Ignore empty messages
		}

		if msgText == "/quit" {
			// Graceful exit
			client.conn.Write([]byte("[System]: Goodbye!\n"))
			break
		}

		// Send message to broadcast channel
		broadcastChan <- Message{
			text:   msgText,
			sender: client,
		}
	}

	// If scanning fails
	if err := scanner.Err(); err != nil {
		log.Printf("Connection error with %s: %v", client.username, err)
	}

	// Notify chat that user has left
	broadcastChan <- Message{
		text:   fmt.Sprintf("%s left the chat", username),
		sender: nil,
	}
}

// Continuously reads from broadcastChan and sends messages to all clients
func broadcastMessages() {
	for msg := range broadcastChan {
		clientsMux.Lock()
		for client := range clients {
			// Don't send the message back to the sender
			if msg.sender != nil && client == msg.sender {
				continue
			}

			// Format message depending on if it was sent by a user or system
			var fullMsg string
			if msg.sender != nil {
				fullMsg = fmt.Sprintf("[%s]: %s\n", msg.sender.username, msg.text)
			} else {
				fullMsg = fmt.Sprintf("[System]: %s\n", msg.text)
			}

			// Attempt to send message; remove client if failed
			if _, err := client.conn.Write([]byte(fullMsg)); err != nil {
				clientLeaveChan <- client
			}
		}
		clientsMux.Unlock()
	}
}

// Manages new clients and disconnections
func manageClients() {
	for {
		select {
		case client := <-clientJoinChan:
			// Add client to map
			clientsMux.Lock()
			clients[client] = true
			clientsMux.Unlock()
			client.conn.Write([]byte(fmt.Sprintf("Welcome %s! Type /quit to exit.\n", client.username)))

		case client := <-clientLeaveChan:
			// Remove client from map and clean up
			clientsMux.Lock()
			if _, ok := clients[client]; ok {
				client.conn.Close()
				delete(clients, client)
				log.Printf("%s disconnected", client.username)
			}
			clientsMux.Unlock()
		}
	}
}
