//Filename: client.go
//Akeylah
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	// Parse command-line flags for host and port (default: 127.0.0.1:4000)
	host := flag.String("host", "127.0.0.1", "Server host")
	port := flag.String("port", "4000", "Server port")
	flag.Parse()

	// Establish a TCP connection to the server
	conn, err := net.Dial("tcp", *host+":"+*port)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close() // Ensure the connection is closed when done

	// Set up readers and writers for connection and terminal input
	reader := bufio.NewReader(conn)        // To read from server
	writer := bufio.NewWriter(conn)        // To send messages to server
	stdin := bufio.NewReader(os.Stdin)     // To read user input

	//Handle Initial Username Prompt
	// Set a read deadline to prevent hanging if server doesn't respond
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read until ':' which ends the username prompt ("Enter username:")
	usernamePrompt, err := reader.ReadString(':')
	if err != nil {
		fmt.Println("Failed to read username prompt:", err)
		return
	}
	fmt.Print(usernamePrompt) // Display prompt to user
	conn.SetReadDeadline(time.Time{}) // Clear the deadline

	// Read user's username input from terminal and send it to the server
	usernameInput, _ := stdin.ReadString('\n')
	usernameInput = strings.TrimSpace(usernameInput)
	writer.WriteString(usernameInput + "\n")
	writer.Flush()

	// Goroutine to listen for server messages and print them 
	go func() {
		for {
			msg, err := reader.ReadString('\n') // Read message from server
			if err != nil {
				fmt.Println("Disconnected from server.")
				os.Exit(0) // Exit the program if connection is lost
			}
			fmt.Print(msg) // Print received message
		}
	}()

	// Main loop to read user input and send to server 
	for {
		input, _ := stdin.ReadString('\n')      // Read from user
		input = strings.TrimSpace(input)        // Remove newline
		if input == "" {
			continue // Ignore empty input
		}
		writer.WriteString(input + "\n")        // Send to server
		writer.Flush()
	}
}

/*Features Included In This Code: 
-Connects to server using --host and --port so the port is configurable
-Receives and responds to a username prompt
-Sends messages from user input to server
-Displays messages from server in real time
-Disconnects cleanly on server error */