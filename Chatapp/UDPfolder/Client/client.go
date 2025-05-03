package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", "localhost:6000")
	localAddr, _ := net.ResolveUDPAddr("udp", ":0") // random port
	conn, _ := net.DialUDP("udp", localAddr, serverAddr)
	defer conn.Close()

	go receiveMessages(conn)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter messages to send:")
	for scanner.Scan() {
		text := scanner.Text()
		if text == "/quit" {
			break
		}
		conn.Write([]byte(text))
	}
}

func receiveMessages(conn *net.UDPConn) {
	buffer := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Connection closed")
			return
		}
		fmt.Println(string(buffer[:n]))
	}
}
