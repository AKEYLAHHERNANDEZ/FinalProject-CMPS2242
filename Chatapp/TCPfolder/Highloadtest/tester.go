//Filename: tester.go
//Akeylah
package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func main() {
	const (
		address       = "localhost:4000"
		username      = "LoadBot"
		totalMessages = 1000
	)

	fmt.Printf("Starting load test with %d messages...\n", totalMessages)

	start := time.Now()

	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Connection error:", err)
		return
	}
	defer conn.Close()

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// Read initial prompt and send username
	rw.ReadString(':') // Read "Enter username:"
	rw.WriteString(username + "\n")
	rw.Flush()

	// Read join message
	rw.ReadString('\n')

	latencies := make([]time.Duration, 0, totalMessages)
	for i := 0; i < totalMessages; i++ {
		msg := fmt.Sprintf("Message #%d from %s", i+1, username)
		before := time.Now()
		rw.WriteString(msg + "\n")
		rw.Flush()

		// We wait for echo/confirmation
		rw.ReadString('\n')
		after := time.Now()
		latencies = append(latencies, after.Sub(before))
	}

	totalTime := time.Since(start)
	avgLatency := averageDuration(latencies)

	fmt.Printf("%s sent %d messages in %.2fs\n", username, totalMessages, totalTime.Seconds())
	fmt.Printf("Throughput: %.2f messages/sec\n", float64(totalMessages)/totalTime.Seconds())
	fmt.Printf("Average latency: %.2f ms\n", avgLatency.Seconds()*1000)
	fmt.Println("Load test completed.")
}

func averageDuration(durations []time.Duration) time.Duration {
	total := time.Duration(0)
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}
