package main

import (
	"fmt"
	"net"
	"sync"
)

// shared registry of channels and subscribed client connections
type PubSub struct {
	channels map[string]map[net.Conn]struct{}
	mu       sync.Mutex
}

func newPubSub() *PubSub {
	return &PubSub{
		channels: make(map[string]map[net.Conn]struct{}),
	}
}

// adds the connection to a channels subscriber set
func (p *PubSub) subscribe(channel string, conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.channels[channel]; !ok {
		p.channels[channel] = make(map[net.Conn]struct{})
	}
	p.channels[channel][conn] = struct{}{}
}

// returns number of clients subscribed to a channel
func (p *PubSub) subscriberCount(channel string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.channels[channel])
}

// acknowledges subscription to a channel
func handleSubscribe(conn net.Conn, args []string, subscriptions map[string]struct{}, pubsub *PubSub) {
	channel := args[1]

	subscriptions[channel] = struct{}{}
	pubsub.subscribe(channel,conn)

	response := fmt.Sprintf(
		"*3\r\n$9\r\nsubscribe\r\n$%d\r\n%s\r\n:%d\r\n",
		len(channel),
		channel,
		len(subscriptions),
	)

	conn.Write([]byte(response))
}

// returns number of clients subscribed to the published channel
func handlePublish(conn net.Conn, args []string, pubsub *PubSub) {
	channel := args[1]
	count := pubsub.subscriberCount(channel)

	conn.Write([]byte(fmt.Sprintf(":%d\r\n", count)))
}
