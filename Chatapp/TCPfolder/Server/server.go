package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
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
	logFile     *os.File
}

type Message struct {
	text   string
	sender *Client
	sentAt time.Time
}

var (
	clients        = make(map[*Client]bool)
	clientsMux     sync.Mutex
	clientJoinChan = make(chan *Client, 100)
	clientLeaveChan = make(chan *Client, 100)
	broadcastChan  = make(chan Message, 1000)
	shutdownChan   = make(chan struct{})
	totalMessages  int
	logDir         = "Log Files"
)

func main() {
	host := flag.String("host", "0.0.0.0", "Server host")
	port := flag.String("port", "4000", "Server port")
	flag.Parse()

	os.MkdirAll(logDir, 0755)
	logFile, err := os.OpenFile(filepath.Join(logDir, "terminal.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error creating log file: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	addr := net.JoinHostPort(*host, *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()
	log.Printf("Server running on %s", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go handleShutdown(cancel, listener)
	go broadcastMessages(ctx)
	go manageClients(ctx)

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

func handleShutdown(cancel context.CancelFunc, listener net.Listener) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down...")
	cancel()
	close(shutdownChan)
	listener.Close()
}

func handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	var username string
	for {
		conn.Write([]byte("Username: "))
		usernameInput, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Username read error: %v", err)
			return
		}
		username = strings.TrimSpace(usernameInput)

		if username == "" {
			conn.Write([]byte("Username cannot be empty.\n"))
			continue
		}

		if isUsernameTaken(username) {
			conn.Write([]byte("Username taken.Enter a unique one below.\n"))
			continue
		}

		break
	}


	logPath := filepath.Join(logDir, username+".log")
	userLog, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	client := &Client{
		username:    username,
		conn:        conn,
		lastActive:  time.Now(),
		messageChan: make(chan string, 10),
		logFile:     userLog,
	}

	clientsMux.Lock()
	clients[client] = true
	count := len(clients)
	clientsMux.Unlock()
	log.Printf("%s joined. Amount Of Online Clients: %d", client.username, count)


	clientJoinChan <- client

	broadcastChan <- Message{
		text:   fmt.Sprintf("%s joined.", username),
		sender: nil,
		sentAt: time.Now(),
	}

	go writeToClient(ctx, client)

	sendUserList(client)

	scanner := bufio.NewScanner(conn)
	buf := make([]byte, 0, 4096)
	scanner.Buffer(buf, 4096)

	inactivityTimeout := 20 * time.Second
	for {
		conn.SetReadDeadline(time.Now().Add(inactivityTimeout))
		if !scanner.Scan() {
			clientLeaveChan <- client
			return
		}
		msg := strings.TrimSpace(scanner.Text())
		if msg == "" {
			continue
		}
		client.lastActive = time.Now()

		if strings.HasPrefix(msg, "/") {
			parts := strings.SplitN(msg, " ", 3)
			cmd := parts[0]

			switch cmd {
			case "/quit":
				client.messageChan <- "Goodbye!"
				clientLeaveChan <- client
				return
			case "/msg":
				if len(parts) < 3 {
					client.messageChan <- "Usage: /msg <username> <message>"
					break
				}
				targetUsername := parts[1]
				privateMsg := parts[2]
				if sendPrivateMessage(client, targetUsername, privateMsg) {
					client.messageChan <- fmt.Sprintf("To %s: %s", targetUsername, privateMsg)
				} else {
					client.messageChan <- fmt.Sprintf("User %s not found.", targetUsername)
				}
			default:
				client.messageChan <- "Unknown command."
			}
			continue
		}

		broadcastChan <- Message{
			text:   msg,
			sender: client,
			sentAt: time.Now(),
		}
	}
}

func writeToClient(ctx context.Context, c *Client) {
	defer c.logFile.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.messageChan:
			if !ok {
				return
			}
			// Remove the extra timestamp here
			formatted := fmt.Sprintf("%s\n", msg)
			c.conn.Write([]byte(formatted))
			c.logFile.WriteString(formatted)
		}
	}
}


func sendUserList(c *Client) {
	clientsMux.Lock()
	defer clientsMux.Unlock()
	var names []string
	for client := range clients {
		names = append(names, client.username)
	}
	c.messageChan <- "Online: " + strings.Join(names, ", ")
}

func broadcastMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-broadcastChan:
			totalMessages++ // Increment totalMessages for each broadcast message
			timestamp := msg.sentAt.Format("15:04:05")
			text := fmt.Sprintf("[%s] %s", timestamp, formatMessage(msg))
			clientsMux.Lock()
			for c := range clients {
				if c != msg.sender {
					select {
					case c.messageChan <- text:
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
		case c := <-clientLeaveChan:
			clientsMux.Lock()
			if _, exists := clients[c]; exists {
				c.conn.Close()
				delete(clients, c)
				close(c.messageChan)
				count := len(clients)
				log.Printf("%s disconnected. Amount Of Online Clients: %d", c.username, count)
				log.Printf("Total messages sent: %d", totalMessages)

				var reason string
				if time.Since(c.lastActive) >= 90*time.Second {
					reason = fmt.Sprintf("%s was disconnected due to inactivity.", c.username)
				} else {
					reason = fmt.Sprintf("%s has left the chat.", c.username)
				}
				broadcastChan <- Message{
					text:   reason,
					sender: nil,
					sentAt: time.Now(),
				}
			}
			clientsMux.Unlock()
		}
	}
}

func isUsernameTaken(name string) bool {
	clientsMux.Lock()
	defer clientsMux.Unlock()
	for c := range clients {
		if strings.EqualFold(c.username, name) {
			return true
		}
	}
	return false
}

func formatMessage(m Message) string {
	if m.sender != nil {
		return fmt.Sprintf("%s: %s", m.sender.username, m.text)
	}
	return m.text
}

func sendPrivateMessage(sender *Client, targetName, msg string) bool {
	clientsMux.Lock()
	defer clientsMux.Unlock()
	for c := range clients {
		if strings.EqualFold(c.username, targetName) {
			timestamp := time.Now().Format("15:04:05")
			formatted := fmt.Sprintf("[%s] Private from %s: %s", timestamp, sender.username, msg)
			c.messageChan <- formatted
			return true
		}
	}
	return false
}