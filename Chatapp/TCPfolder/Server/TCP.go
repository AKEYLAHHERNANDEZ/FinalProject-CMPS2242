package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
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

	// Start services
	go broadcastMessages(ctx)
	go manageClients(ctx)
	go monitorConnections(ctx)

	// Accept connections
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

// clientWriter handles writing messages to a single client
func clientWriter(ctx context.Context, client *Client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	defer close(client.messageChan)

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-client.messageChan:
			if !ok {
				return // Channel closed
			}
			if _, err := client.conn.Write([]byte(msg + "\n")); err != nil {
				clientLeaveChan <- client
				return
			}
		case <-ticker.C:
			if _, err := client.conn.Write([]byte("PING\n")); err != nil {
				clientLeaveChan <- client
				return
			}
		}
	}
}

func handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	// Get username
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	conn.Write([]byte("Enter username (3-20 alphanumeric chars): "))

	username, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		log.Printf("Username read error: %v", err)
		return
	}
	username = strings.TrimSpace(username)

	if !isValidUsername(username) || isUsernameTaken(username) {
		conn.Write([]byte("Invalid or taken username. Disconnecting.\n"))
		return
	}

	// Create client
	client := &Client{
		username:    username,
		conn:        conn,
		lastActive:  time.Now(),
		messageChan: make(chan string, 10),
	}

	// Register client
	select {
	case clientJoinChan <- client:
	case <-ctx.Done():
		return
	}

	// Notify chat
	broadcastChan <- Message{
		text:   fmt.Sprintf("%s joined", username),
		sender: nil,
		sentAt: time.Now(),
	}

	// Start writer
	go clientWriter(ctx, client)

	// Read messages
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			msgText := strings.TrimSpace(scanner.Text())
			client.lastActive = time.Now()

			if msgText == "" {
				continue
			}

			if msgText == "/quit" {
				client.messageChan <- "Goodbye!"
				break
			}

			if len(msgText) > maxMessageLength {
				client.messageChan <- fmt.Sprintf("Message exceeds %d chars", maxMessageLength)
				continue
			}

			broadcastChan <- Message{
				text:   msgText,
				sender: client,
				sentAt: time.Now(),
			}
		}
	}

	// Cleanup
	broadcastChan <- Message{
		text:   fmt.Sprintf("%s left", username),
		sender: nil,
		sentAt: time.Now(),
	}
}

func broadcastMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-broadcastChan:
			clientsMux.Lock()
			totalMessages++
			recipients := make([]*Client, 0, len(clients))
			for c := range clients {
				if c != msg.sender {
					recipients = append(recipients, c)
				}
			}
			clientsMux.Unlock()

			// Format message
			var prefix string
			if msg.sender != nil {
				prefix = fmt.Sprintf("[%s]: ", msg.sender.username)
			} else {
				prefix = "[System]: "
			}

			// Deliver with simulated network delay (0-200ms)
			delay := time.Duration(rand.Intn(200)) * time.Millisecond
			time.Sleep(delay)

			// Simulate 10% packet loss
			if rand.Float32() < 0.1 {
				continue
			}

			for _, c := range recipients {
				select {
				case c.messageChan <- prefix + msg.text:
				default:
					clientLeaveChan <- c
				}
			}
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
			client.messageChan <- fmt.Sprintf("Welcome %s! Type /quit to exit.", client.username)

		case client := <-clientLeaveChan:
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

func monitorConnections(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
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

func isValidUsername(username string) bool {
	if len(username) < 3 || len(username) > 20 {
		return false
	}
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
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
