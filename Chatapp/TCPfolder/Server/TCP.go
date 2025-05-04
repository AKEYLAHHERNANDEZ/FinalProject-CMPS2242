package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Client struct {
	username    string
	conn        net.Conn
	lastActive  time.Time
	messageChan chan string
}

type Message struct {
	text   string
	sender *Client
	sentAt time.Time
}

var (
	clients          = make(map[*Client]bool)
	clientsMux       sync.Mutex
	clientJoinChan   = make(chan *Client, 100)
	clientLeaveChan  = make(chan *Client, 100)
	broadcastChan    = make(chan Message, 1000)
	shutdownChan     = make(chan struct{})
	maxMessageLength = 512
	pingInterval     = 30 * time.Second
	totalMessages    int
)

func main() {
	host := flag.String("host", "0.0.0.0", "Server host address")
	port := flag.String("port", "4000", "Server port number")
	flag.Parse()

	addr := net.JoinHostPort(*host, *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()
	log.Printf("Chat server running on %s", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		cancel()
		close(shutdownChan)
		listener.Close()
	}()

	go broadcastMessages(ctx)
	go manageClients(ctx)
	go monitorConnections(ctx)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-shutdownChan:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}
		go handleConnection(ctx, conn)
	}
}

func handleConnection(ctx context.Context, conn net.Conn) {
		defer conn.Close()
	
		// Immediately request username
		conn.SetDeadline(time.Now().Add(50 * time.Second))
		_, err := conn.Write([]byte("Enter username: "))
		if err != nil {
			log.Printf("Username prompt error: %v", err)
			return
		}
	
		// Read username first before anything else
		username, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Printf("Username read error: %v", err)
			return
		}
		username = strings.TrimSpace(username)
	
		if isUsernameTaken(username) {
			conn.Write([]byte("Username taken. Disconnecting.\n"))
			return
		}

	client := &Client{
		username:    username,
		conn:        conn,
		lastActive:  time.Now(),
		messageChan: make(chan string, 10),
	}

	log.Printf("Client joined: %s", username)

	select {
	case clientJoinChan <- client:
	case <-ctx.Done():
		return
	}

	broadcastChan <- Message{
		text:   fmt.Sprintf("%s joined", username),
		sender: nil,
		sentAt: time.Now(),
	}

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-client.messageChan:
				if !ok {
					return
				}
				_, err := conn.Write([]byte(msg + "\n"))
				if err != nil {
					clientLeaveChan <- client
					return
				}
			case <-ticker.C:
				if _, err := conn.Write([]byte("PING\n")); err != nil {
					clientLeaveChan <- client
					return
				}
			}
		}
	}()

	// Increase buffer for long messages
	scanner := bufio.NewScanner(conn)
	buf := make([]byte, 0, 2048)
	scanner.Buffer(buf, 4096)

	for scanner.Scan() {
		msgText := strings.TrimSpace(scanner.Text())
		client.lastActive = time.Now()

		if msgText == "" {
			continue
		}

		// Command handling
		if strings.HasPrefix(msgText, "/") {
			args := strings.Fields(msgText)
			command := args[0]

			switch command {
			case "/quit":
				client.messageChan <- "Goodbye!"
				clientLeaveChan <- client
				return
			case "/users":
				client.messageChan <- listUsernames()
			default:
				client.messageChan <- "Unknown command."
			}
			continue
		}

		if len(msgText) > maxMessageLength {
			client.messageChan <- fmt.Sprintf("Message exceeds %d characters", maxMessageLength)
			continue
		}

		broadcastChan <- Message{
			text:   msgText,
			sender: client,
			sentAt: time.Now(),
		}
	}

	broadcastChan <- Message{
		text:   fmt.Sprintf("%s left", username),
		sender: nil,
		sentAt: time.Now(),
	}

	clientLeaveChan <- client
	log.Printf("Client left: %s", username)
}

func broadcastMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-broadcastChan:
			clientsMux.Lock()
			totalMessages++
			for c := range clients {
				if c != msg.sender {
					select {
					case c.messageChan <- formatMessage(msg):
					default:
						clientLeaveChan <- c
					}
				}
			}
			clientsMux.Unlock()
		}
	}
}

func manageClients(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-clientJoinChan:
			clientsMux.Lock()
			clients[client] = true
			clientsMux.Unlock()
			client.messageChan <- fmt.Sprintf("Welcome %s!", client.username)
		case client := <-clientLeaveChan:
			clientsMux.Lock()
			if _, ok := clients[client]; ok {
				client.conn.Close()
				delete(clients, client)
				close(client.messageChan)
			}
			clientsMux.Unlock()
		}
	}
}

func monitorConnections(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			clientsMux.Lock()
			for client := range clients {
				if time.Since(client.lastActive) > 2*pingInterval {
					client.messageChan <- "[System]: Disconnecting due to inactivity"
					clientLeaveChan <- client
				}
			}
			log.Printf("Active clients: %d | Total messages: %d", len(clients), totalMessages)
			clientsMux.Unlock()
		}
	}
}

func formatMessage(msg Message) string {
	if msg.sender != nil {
		return fmt.Sprintf("[%s]: %s", msg.sender.username, msg.text)
	}
	return "[System]: " + msg.text
}

func isUsernameTaken(username string) bool {
	clientsMux.Lock()
	defer clientsMux.Unlock()
	for client := range clients {
		if strings.EqualFold(client.username, username) {
			return true
		}
	}
	return false
}

func listUsernames() string {
	clientsMux.Lock()
	defer clientsMux.Unlock()
	var usernames []string
	for client := range clients {
		usernames = append(usernames, client.username)
	}
	return "Online: " + strings.Join(usernames, ", ")
}