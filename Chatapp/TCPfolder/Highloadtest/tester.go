//Filename: tester.go
//Akeylah
package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:4000") 
	if err != nil {
		fmt.Println("Connection error:", err)
		return
	}
	defer conn.Close()

	// Send username directly
	conn.Write([]byte("LoadBot\n"))

	// Rapid message sending
	for i := 0; i < 1000; i++ {
		msg := fmt.Sprintf("Message #%d from LoadBot", i)
		_, err := conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Println("Write error:", err)
			break
		}
		time.Sleep(10 * time.Millisecond) // Simulate message flood
	}

	fmt.Println("Load test completed.")
}
