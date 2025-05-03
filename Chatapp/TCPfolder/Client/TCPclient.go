package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	// Configure command-line flags
	serverHost := flag.String("host", "localhost", "Server host address")
	serverPort := flag.String("port", "4000", "Server port number")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to server using configured host:port
	serverAddr := net.JoinHostPort(*serverHost, *serverPort)
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Printf("Connection failed to %s: %v\n", serverAddr, err)
		return
	}
	defer conn.Close()

	fmt.Printf("Connected to server at %s. Type /quit to exit.\n", serverAddr)

	// Buffered channels
	msgChan := make(chan string, 10)
	errChan := make(chan error, 1)

	// Reader goroutine
	go func() {
		reader := bufio.NewReader(conn)
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				errChan <- fmt.Errorf("server disconnected: %v", err)
				return
			}
			msgChan <- strings.TrimSpace(msg)
		}
	}()

	// Writer goroutine
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
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

	// Main loop
	for {
		select {
		case msg := <-msgChan:
			if msg == "PING" {
				fmt.Fprintln(conn, "PONG") // Respond to keepalive
				continue
			}
			fmt.Println(msg)

		case err := <-errChan:
			fmt.Printf("Error: %v\n", err)
			return

		case <-sigChan:
			fmt.Fprintln(conn, "/quit")
			time.Sleep(100 * time.Millisecond)
			return

		case <-ctx.Done():
			return
		}
	}
}
