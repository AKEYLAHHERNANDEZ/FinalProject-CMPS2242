package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	for {
		err := startClient()
		if err != nil {
			fmt.Println("Disconnected. Reconnecting in 3 seconds...")
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}
}

func startClient() error {
	serverAddr, _ := net.ResolveUDPAddr("udp", "localhost:6000")
	localAddr, _ := net.ResolveUDPAddr("udp", ":0") // random port
	conn, err := net.DialUDP("udp", localAddr, serverAddr)
	if err != nil {
		fmt.Println("Failed to connect:", err)
		return err
	}
	defer conn.Close()

	// Start receiving and heartbeat routines
	go receiveMessages(conn)
	go sendHeartbeat(conn)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Commands: (/quit to exit) or (/msg <username><message> for private message)")
	fmt.Println("Enter messages to send:")

	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "/quit" {
			fmt.Println("Exiting client.")
			return nil
		}
		_, err := conn.Write([]byte(text))
		if err != nil {
			fmt.Println("Error sending message:", err)
			return err
		}
	}

	return nil
}

func receiveMessages(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Server closed the connection.")
			return
		}
		fmt.Println(string(buffer[:n]))
	}
}

func sendHeartbeat(conn *net.UDPConn) {
	for {
		time.Sleep(10 * time.Second)
		_, err := conn.Write([]byte("/ping"))
		if err != nil {
			return
		}
	}
}
