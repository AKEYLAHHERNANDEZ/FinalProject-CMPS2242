package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type Client struct {
	username string
	conn     net.Conn
}

type Message struct {
	text   string
	sender *Client
}

var (
	clients         = make(map[*Client]bool)
	clientsMux      sync.Mutex
	clientJoinChan  = make(chan *Client)
	clientLeaveChan = make(chan *Client)
	broadcastChan   = make(chan Message)
)



func main() {
    // Configuration
    host := flag.String("host", "0.0.0.0", "Server host address")
    port := flag.String("port", "4000", "Server port number")
    flag.Parse()


    // Start server
    addr := net.JoinHostPort(*host, *port)
    listener, err := net.Listen("tcp", addr)
    if err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
    defer listener.Close()
    log.Printf("TCP Chat Server running on %s", addr)


    // Graceful shutdown
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        <-sigChan
        log.Println("Shutting down server...")
        listener.Close() // This will break the Accept() loop
    }()


    // Start service goroutines
    go broadcastMessages()
    go manageClients()


    // Accept connections
    for {
        conn, err := listener.Accept()
        if err != nil {
            if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
                break // Normal shutdown
            }
            log.Printf("Accept error: %v", err)
            continue
        }
        go handleConnection(conn)
    }


    // Wait for all clients to disconnect
    clientsMux.Lock()
    for client := range clients {
        client.conn.Close()
        delete(clients, client)
    }
    clientsMux.Unlock()
}
