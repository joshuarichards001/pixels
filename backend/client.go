package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

// Client wraps a [websocket.Conn] to add thread safety to
// WriteMessage.
type Client struct {
	*websocket.Conn
	Addr netip.Addr

	m sync.Mutex
}

func (c *Client) WriteMessage(messageType int, data []byte) error {
	c.m.Lock()
	defer c.m.Unlock()

	return c.Conn.WriteMessage(messageType, data)
}

type registration struct {
	client *Client
	rsp    chan error
}

type broadcast struct {
	src *Client
	msg IncomingMessage
}

type rateLimitData struct {
	clients int
	limiter *rate.Limiter
}

type clientManager struct {
	running atomic.Bool
	done    chan struct{}

	broadcast  chan broadcast
	register   chan registration
	unregister chan *Client
	numclients chan int

	clients    map[*Client]struct{}
	rateLimits map[netip.Addr]*rateLimitData

	redisClient *redis.Client
}

func newClientManager(rc *redis.Client) *clientManager {
	return &clientManager{
		done:       make(chan struct{}),
		broadcast:  make(chan broadcast),
		register:   make(chan registration),
		unregister: make(chan *Client),
		numclients: make(chan int),

		clients:    make(map[*Client]struct{}),
		rateLimits: make(map[netip.Addr]*rateLimitData),

		redisClient: rc,
	}
}

// Run runs the clientManager. It blocks until the manager exits,
// which happens once the context is canceled. Calling this method
// more than once, even after the manager has exited, will panic.
func (cm *clientManager) Run(ctx context.Context) {
	if cm.running.Swap(true) {
		panic(errors.New("manager has already been run"))
	}
	defer close(cm.done)

	for {
		select {
		case <-ctx.Done():
			return

		case update := <-cm.broadcast:
			cm.send(ctx, update)

		case r := <-cm.register:
			r.rsp <- cm.add(r.client)

		case client := <-cm.unregister:
			cm.remove(client)

		case cm.numclients <- len(cm.clients):
		}
	}
}

func (cm *clientManager) send(ctx context.Context, update broadcast) {
	if !cm.rateLimits[update.src.Addr].limiter.Allow() {
		update.src.WriteMessage(websocket.TextMessage, []byte("rate limit exceeded"))
		return
	}

	log.Printf("Pixel updated: index=%d, color=%s, ip=%s", update.msg.Data.Index, update.msg.Data.Color, update.src.Addr)

	err := cm.redisClient.SetRange(ctx, "pixels", int64(update.msg.Data.Index), update.msg.Data.Color).Err()
	if err != nil {
		log.Printf("error updating Redis: %v", err)
		return
	}

	msg := OutgoingMessage{Type: "update", Data: update.msg.Data, ClientCount: len(cm.clients)}
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Printf("error marshaling json: %v", err)
		return
	}

	for client := range cm.clients {
		err := client.WriteMessage(websocket.TextMessage, jsonMsg)
		if err != nil {
			log.Printf("error sending message to client: %v", err)
			cm.remove(client)
		}
	}
}

func (cm *clientManager) add(client *Client) error {
	limit := cm.rateLimits[client.Addr]
	if limit == nil {
		limit = &rateLimitData{
			clients: 1, limiter: rate.NewLimiter(rate.Every(time.Second), 5),
		}
		cm.rateLimits[client.Addr] = limit
		return nil
	}
	if limit.clients >= 5 {
		return errors.New("too many clients with IP")
	}
	limit.clients++
	cm.clients[client] = struct{}{}

	return nil
}

func (cm *clientManager) remove(client *Client) {
	if _, ok := cm.clients[client]; !ok {
		return
	}

	delete(cm.clients, client)
	client.Close()

	limit := cm.rateLimits[client.Addr]
	limit.clients--
	if limit.clients <= 0 {
		delete(cm.rateLimits, client.Addr)
	}
}

func (cm *clientManager) Register(client *Client) error {
	rsp := make(chan error)
	select {
	case <-cm.done:
		return errors.New("client manager has exited")
	case cm.register <- registration{client: client, rsp: rsp}:
	}
	return <-rsp
}

func (cm *clientManager) Unregister(client *Client) {
	select {
	case <-cm.done:
	case cm.unregister <- client:
	}
}

func (cm *clientManager) Broadcast(src *Client, msg IncomingMessage) {
	select {
	case <-cm.done:
	case cm.broadcast <- broadcast{src: src, msg: msg}:
	}
}

func (cm *clientManager) Num() int {
	select {
	case <-cm.done:
		return 0
	case num := <-cm.numclients:
		return num
	}
}
