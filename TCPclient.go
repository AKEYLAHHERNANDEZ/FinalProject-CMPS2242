package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:4000")
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer conn.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	doneChan := make(chan bool)

	go func() {
		<-sigChan
		fmt.Println("\nDisconnecting from server...")
		conn.Write([]byte("/quit\n"))
		doneChan <- true
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	// Handle incoming messages from the server
	go func() {
		defer wg.Done()
		reader := bufio.NewReader(conn)
		for {
			message, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Disconnected from server: %v\n", err)
				doneChan <- true
				return
			}
			fmt.Print(message)
		}
	}()

	// Send messages to the server
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			text := scanner.Text()
			_, err := fmt.Fprintf(conn, text+"\n")
			if err != nil {
				fmt.Printf("Error sending message: %v\n", err)
				doneChan <- true
				break
			}

			if text == "/quit" {
				doneChan <- true
				return
			}
		}
	}()

	// Wait for graceful exit
	<-doneChan
	wg.Wait()
}
