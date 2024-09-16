package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/netip"
	"sync"
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

type clientManager struct {
	done       chan struct{}
	broadcast  chan broadcast
	register   chan registration
	unregister chan *Client
	numclients chan int

	redisClient *redis.Client
}

func newClientManager(rc *redis.Client) *clientManager {
	return &clientManager{
		done:       make(chan struct{}),
		broadcast:  make(chan broadcast),
		register:   make(chan registration),
		unregister: make(chan *Client),
		numclients: make(chan int),

		redisClient: rc,
	}
}

func (cm *clientManager) Run(ctx context.Context) {
	select {
	case <-cm.done:
		panic("manager has already been run")
	default:
	}
	defer close(cm.done)

	clients := make(map[*Client]struct{})

	type rateLimitData struct {
		clients int
		limiter *rate.Limiter
	}
	rateLimits := make(map[netip.Addr]*rateLimitData)

	for {
		select {
		case <-ctx.Done():
			return

		case update := <-cm.broadcast:
			if !rateLimits[update.src.Addr].limiter.Allow() {
				update.src.WriteMessage(websocket.TextMessage, []byte("rate limit exceeded"))
				continue
			}

			log.Printf("Pixel updated: index=%d, color=%s, ip=%s", update.msg.Data.Index, update.msg.Data.Color, update.src.Addr)

			err := cm.redisClient.SetRange(ctx, "pixels", int64(update.msg.Data.Index), update.msg.Data.Color).Err()
			if err != nil {
				log.Printf("error updating Redis: %v", err)
				continue
			}

			msg := OutgoingMessage{Type: "update", Data: update.msg.Data, ClientCount: len(clients)}
			jsonMsg, err := json.Marshal(msg)
			if err != nil {
				log.Printf("error marshaling json: %v", err)
				continue
			}

			for client := range clients {
				err := client.WriteMessage(websocket.TextMessage, jsonMsg)
				if err != nil {
					log.Printf("error sending message to client: %v", err)
					client.Close()
					delete(clients, client)
				}
			}

		case r := <-cm.register:
			client := r.client
			limit := rateLimits[client.Addr]
			if limit == nil {
				limit = &rateLimitData{
					clients: 1, limiter: rate.NewLimiter(rate.Every(time.Second), 5),
				}
				rateLimits[client.Addr] = limit
				continue
			}
			if limit.clients >= 5 {
				r.rsp <- errors.New("too many clients with IP")
				continue
			}
			limit.clients++
			clients[client] = struct{}{}

			r.rsp <- nil

		case client := <-cm.unregister:
			if _, ok := clients[client]; !ok {
				continue
			}

			delete(clients, client)
			client.Close()

			limit := rateLimits[client.Addr]
			limit.clients--
			if limit.clients <= 0 {
				delete(rateLimits, client.Addr)
			}

		case cm.numclients <- len(clients):
		}
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
