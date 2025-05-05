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
	host := flag.String("host", "127.0.0.1", "Server host")
	port := flag.String("port", "4000", "Server port")
	flag.Parse()

	conn, err := net.Dial("tcp", *host+":"+*port)
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	stdin := bufio.NewReader(os.Stdin)

	// Read the initial server prompt for username and print it before anything else
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	usernamePrompt, err := reader.ReadString(':')
	if err != nil {
		fmt.Println("Failed to read username prompt:", err)
		return
	}
	fmt.Print(usernamePrompt)
	conn.SetReadDeadline(time.Time{}) // Clear deadline

	usernameInput, _ := stdin.ReadString('\n')
	usernameInput = strings.TrimSpace(usernameInput)
	writer.WriteString(usernameInput + "\n")
	writer.Flush()

	go func() {
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Disconnected from server.")
				os.Exit(0)
			}
			fmt.Print(msg)
		}
	}()

	for {
		input, _ := stdin.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		writer.WriteString(input + "\n")
		writer.Flush()
	}
}
