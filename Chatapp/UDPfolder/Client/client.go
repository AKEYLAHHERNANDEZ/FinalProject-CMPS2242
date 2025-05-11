package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	host := flag.String("host", "127.0.0.1", "Server host")
	port := flag.String("port", "4000", "Server port")
	flag.Parse()

	// Resolve the server's address
	serverAddr := fmt.Sprintf("%s:%s", *host, *port)
	udpAddr, err := net.ResolveUDPAddr("udp", serverAddr)
	if err != nil {
		fmt.Printf("Failed to resolve UDP address: %v\n", err)
		return
	}

	// Connect to the server via UDP
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer conn.Close()

	// Read user input for the username
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your username: ")
	usernameInput, _ := reader.ReadString('\n')
	usernameInput = strings.TrimSpace(usernameInput)

	// Send username to the server
	sendMessage(conn, usernameInput)

	// Listen for messages from the server in a separate goroutine
	go listenForMessages(conn)

	// Send user messages
	for {
		fmt.Print("Enter message (or /msg <username> <message> for private message, /clients to see online clients, /quit to exit): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "/quit" {
			sendMessage(conn, "/quit")
			break
		}

		if input == "/clients" {
			sendMessage(conn, "/clients")
		} else if input != "" {
			// If the input starts with "/msg", treat it as a private message
			if strings.HasPrefix(input, "/msg") {
				sendPrivateMessage(conn, input)
			} else {
				sendMessage(conn, input) // Send regular message
			}
		}
	}
}

// Function to send a message to the server
func sendMessage(conn *net.UDPConn, msg string) {
	_, err := conn.Write([]byte(msg + "\n"))
	if err != nil {
		fmt.Println("Error sending message:", err)
	}
}

// Function to send a private message to a specific client
func sendPrivateMessage(conn *net.UDPConn, msg string) {
	_, err := conn.Write([]byte(msg + "\n"))
	if err != nil {
		fmt.Println("Error sending private message:", err)
	}
}

// Function to listen for incoming messages from the server
func listenForMessages(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from server:", err)
			return
		}
		fmt.Printf("Received message: %s\n", string(buffer[:n]))
	}
}
