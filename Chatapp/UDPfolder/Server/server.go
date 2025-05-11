package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

type Client struct {
	username   string
	conn       *net.UDPConn
	clientAddr *net.UDPAddr
}

var (
	clients    = make(map[string]*Client) // Mapping of client usernames to their UDP addresses
	clientsMux sync.Mutex
	logFile    *os.File
)

func main() {
	host := flag.String("host", "0.0.0.0", "Server host")
	port := flag.String("port", "4000", "Server port")
	flag.Parse()

	// Set up server UDP address
	serverAddr := fmt.Sprintf("%s:%s", *host, *port)
	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		log.Fatalf("Failed to resolve UDP address: %v", err)
	}

	// Create UDP listener
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("Failed to listen on UDP: %v", err)
	}
	defer conn.Close()

	// Open log file
	logFile, err = os.OpenFile("server_log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file: %v", err)
	}
	defer logFile.Close()

	log.Printf("Server is listening on %s", serverAddr)

	// Listen for incoming messages
	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Println("Error reading from UDP:", err)
			continue
		}

		message := string(buffer[:n])
		handleClientMessage(conn, clientAddr, message)
	}
}

// Handle incoming client messages
func handleClientMessage(conn *net.UDPConn, clientAddr *net.UDPAddr, message string) {
	message = strings.TrimSpace(message)

	// Register a new client if it's their first message (username)
	if _, exists := clients[clientAddr.String()]; !exists {
		clientsMux.Lock()
		clients[clientAddr.String()] = &Client{
			username:   message,
			conn:       conn,
			clientAddr: clientAddr,
		}
		clientsMux.Unlock()
		log.Printf("%s joined the chat", message)

		// Welcome the new client
		conn.WriteToUDP([]byte("Welcome "+message+"! Type your messages below.\n"), clientAddr)

		// Log the client joining
		logMessage(fmt.Sprintf("%s joined the chat", message))
	}

	// If it's not a username message, it's a regular or private message
	client := clients[clientAddr.String()]
	if client != nil {
		// Check if it's a private message
		if strings.HasPrefix(message, "/msg") {
			handlePrivateMessage(conn, client, message)
		} else if message == "/quit" {
			handleClientQuit(conn, client)
		} else if message == "/clients" {
			handleClientList(conn, client)
		} else {
			// Broadcast to all other clients
			broadcastMessage(conn, client, message)
		}
	}
}

// Handle private messages
func handlePrivateMessage(conn *net.UDPConn, sender *Client, message string) {
	parts := strings.SplitN(message, " ", 3)
	if len(parts) < 3 {
		conn.WriteToUDP([]byte("Usage: /msg <username> <message>\n"), sender.clientAddr)
		return
	}
	targetUsername := parts[1]
	privateMessage := parts[2]

	// Look for the target client by username
	clientsMux.Lock()
	targetClient, exists := findClientByUsername(targetUsername)
	clientsMux.Unlock()

	if !exists {
		conn.WriteToUDP([]byte(fmt.Sprintf("User %s not found.\n", targetUsername)), sender.clientAddr)
		return
	}

	// Send the private message to the target client
	formattedMessage := fmt.Sprintf("Private message from %s: %s\n", sender.username, privateMessage)
	conn.WriteToUDP([]byte(formattedMessage), targetClient.clientAddr)
	conn.WriteToUDP([]byte(fmt.Sprintf("Private message sent to %s: %s\n", targetUsername, privateMessage)), sender.clientAddr)
}

// Broadcast message to all clients except the sender
func broadcastMessage(conn *net.UDPConn, sender *Client, message string) {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	for _, client := range clients {
		// Don't send the message back to the sender
		if client != sender {
			formattedMessage := fmt.Sprintf("%s: %s\n", sender.username, message)
			conn.WriteToUDP([]byte(formattedMessage), client.clientAddr)
		}
	}
}

// Handle client exit
func handleClientQuit(conn *net.UDPConn, client *Client) {
	clientsMux.Lock()
	delete(clients, client.clientAddr.String())
	clientsMux.Unlock()

	// Notify all clients
	broadcastMessage(conn, client, fmt.Sprintf("%s has left the chat", client.username))

	// Log the client quitting
	logMessage(fmt.Sprintf("%s has left the chat", client.username))
}

// Handle client list request
func handleClientList(conn *net.UDPConn, client *Client) {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	var clientList string
	for _, c := range clients {
		clientList += c.username + "\n"
	}

	if clientList == "" {
		clientList = "No clients are online.\n"
	}

	conn.WriteToUDP([]byte("Online clients:\n"+clientList), client.clientAddr)
	logMessage(fmt.Sprintf("Client %s requested the client list.", client.username))
}

// Find a client by their username
func findClientByUsername(username string) (*Client, bool) {
	for _, client := range clients {
		if client.username == username {
			return client, true
		}
	}
	return nil, false
}

// Log messages to the server log file
func logMessage(msg string) {
	log.SetOutput(logFile)
	log.Println(msg)
}
