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
	"syscall"
	"time"
)

func main() {
	serverHost := flag.String("host", "localhost", "Server host address")
	serverPort := flag.String("port", "4000", "Server port number")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect with timeout settings
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 1 * time.Minute,
	}
	serverAddr := net.JoinHostPort(*serverHost, *serverPort)
	conn, err := dialer.Dial("tcp", serverAddr)
	if err != nil {
		log.Fatalf("Connection failed to %s: %v\n", serverAddr, err)
	}
	defer conn.Close()

	// Enable TCP keepalive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
	}

	log.Printf("Connected to server at %s (Type /quit to exit).\n", serverAddr)

	// Handle username exchange first
	reader := bufio.NewReader(conn)
	prompt, err := reader.ReadString(':')
	if err != nil {
		log.Fatalf("Failed to read username prompt: %v", err)
	}
	fmt.Print(strings.TrimSpace(prompt) + " ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	username := strings.TrimSpace(scanner.Text())
	fmt.Fprintln(conn, username)

	// Setup communication channels
	msgChan := make(chan string, 10)
	errChan := make(chan error, 1)

	// Message receiver
	go func() {
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				errChan <- fmt.Errorf("server disconnected: %v", err)
				return
			}
			msg = strings.TrimSpace(msg)
			
			if msg == "__PING__" {
				fmt.Fprintln(conn, "__PONG__")
				continue
			}
			
			msgChan <- msg
		}
	}()

	// Message sender
	go func() {
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}

			if text == "/quit" {
				fmt.Fprintln(conn, text)
				cancel()
				return
			}

			if _, err := fmt.Fprintln(conn, text); err != nil {
				errChan <- err
				return
			}
		}
	}()

	// Signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case msg := <-msgChan:
			fmt.Println(msg)
		case err := <-errChan:
			fmt.Printf("Error: %v\n", err)
			return
		case <-sigChan:
			fmt.Fprintln(conn, "/quit")
			return
		case <-ctx.Done():
			return
		}
	}
}