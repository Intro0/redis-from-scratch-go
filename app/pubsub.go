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

// removes a client from a channels subscriber set
func (p *PubSub) unsubscribe(channel string, conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	subscribers, ok := p.channels[channel]
	if !ok {
		return
	}
	delete(subscribers, conn)

	if len(subscribers) == 0 {
		delete(p.channels, channel)
	}
}

// returns list of connections subscribed to a channel
func (p *PubSub) subscribers(channel string) []net.Conn {
	p.mu.Lock()
	defer p.mu.Unlock()

	subscribers := make([]net.Conn, 0, len(p.channels[channel]))
	for conn := range p.channels[channel] {
		subscribers = append(subscribers, conn)
	}

	return subscribers
}

// acknowledges subscription to a channel
func handleSubscribe(conn net.Conn, args []string, subscriptions map[string]struct{}, pubsub *PubSub) {
	channel := args[1]

	subscriptions[channel] = struct{}{}
	pubsub.subscribe(channel, conn)

	response := fmt.Sprintf(
		"*3\r\n$9\r\nsubscribe\r\n$%d\r\n%s\r\n:%d\r\n",
		len(channel),
		channel,
		len(subscriptions),
	)

	conn.Write([]byte(response))
}

// acknowledges unsubscription from a channel
func handleUnsubscribe(conn net.Conn, args []string, subscriptions map[string]struct{}, pubsub *PubSub) {
	channel := args[1]
	delete(subscriptions, channel)
	pubsub.unsubscribe(channel, conn)

	response := fmt.Sprintf(
		"*3\r\n$11\r\nunsubscribe\r\n$%d\r\n%s\r\n:%d\r\n",
		len(channel),
		channel,
		len(subscriptions),
	)

	conn.Write([]byte(response))
}

// returns number of clients subscribed to the published channel
func handlePublish(conn net.Conn, args []string, pubsub *PubSub) {
	channel := args[1]
	message := args[2]

	subscribers := pubsub.subscribers(channel)

	response := fmt.Sprintf(
		"*3\r\n$7\r\nmessage\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
		len(channel),
		channel,
		len(message),
		message,
	)

	for _, sub := range subscribers {
		sub.Write([]byte(response))
	}

	fmt.Fprintf(conn, ":%d\r\n", len(subscribers))
}
