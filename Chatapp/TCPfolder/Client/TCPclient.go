package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to server
	conn, err := net.Dial("tcp", "localhost:4000")
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	// Buffered channel for messages
	msgChan := make(chan string, 10) // Buffer to prevent blocking
	errChan := make(chan error, 1)

	// Start reader goroutine
	go func() {
		reader := bufio.NewReader(conn)
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				errChan <- fmt.Errorf("connection error: %v", err)
				return
			}
			msgChan <- strings.TrimSpace(msg)
		}
	}()

	// Start writer goroutine
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}

			if _, err := fmt.Fprintln(conn, text); err != nil {
				errChan <- err
				return
			}

			if text == "/quit" {
				cancel()
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
			fmt.Println(msg) // Print all incoming messages

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