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
