package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

func newServer() *Server {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDRESS"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	return &Server{
		broadcast:   make(chan IncomingMessage),
		register:    make(chan *websocket.Conn),
		unregister:  make(chan *websocket.Conn),
		redisClient: rdb,
		ctx:         ctx,
	}
}

func (server *Server) run() {
	for {
		select {
		case client := <-server.register:
			server.clients.Store(client, true)
		case client := <-server.unregister:
			server.clients.Delete(client)
			client.Close()
		case update := <-server.broadcast:
			err := server.redisClient.SetRange(server.ctx, "pixels", int64(update.Data.Index), update.Data.Color).Err()
			if err != nil {
				log.Printf("error updating Redis: %v", err)
				continue
			}

			log.Printf("Pixel updated: index=%d, color=%s", update.Data.Index, update.Data.Color)

			dataCopy, err := server.redisClient.Get(server.ctx, "pixels").Result()
			if err != nil {
				log.Printf("error getting data from Redis: %v", err)
				continue
			}

			clientCount := server.countClients()

			msg := OutgoingMessage{Type: "update", Data: dataCopy, ClientCount: clientCount}
			jsonMsg, err := json.Marshal(msg)
			if err != nil {
				log.Printf("error marshaling json: %v", err)
				continue
			}

			server.clients.Range(func(key, value interface{}) bool {
				client := key.(*websocket.Conn)
				err := client.WriteMessage(websocket.TextMessage, jsonMsg)
				if err != nil {
					log.Printf("error sending message to client: %v", err)
					client.Close()
					server.clients.Delete(client)
				}
				return true
			})
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64,
	WriteBufferSize: 10240,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		environment := os.Getenv("ENVIRONMENT")
		if environment == "development" {
			return origin == "http://127.0.0.1:5500"
		} else {
			return origin == "https://tenthousandpixels.com"
		}
	},
}

func (server *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error upgrading connection: %v", err)
		http.Error(w, "could not open websocket connection", http.StatusInternalServerError)
		return
	}

	server.register <- conn

	defer func() {
		server.unregister <- conn
		conn.Close()
	}()

	initialData, err := server.redisClient.Get(server.ctx, "pixels").Result()
	if err != nil {
		log.Printf("error getting initial data from Redis: %v", err)
		return
	}

	clientCount := server.countClients()

	initialMsg := OutgoingMessage{Type: "initial", Data: initialData, ClientCount: clientCount}
	jsonMsg, err := json.Marshal(initialMsg)
	if err != nil {
		log.Printf("error marshaling initial JSON: %v", err)
		return
	}

	err = conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("error sending initial message: %v", err)
		return
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error reading message: %v", err)
			}
			break
		}

		var update IncomingMessage
		err = json.Unmarshal(msgBytes, &update)
		if err != nil {
			log.Printf("error unmarshaling JSON: %v", err)
			conn.WriteMessage(websocket.TextMessage, []byte("Invalid input type"))
			continue
		}

		if err := validateIncomingMessage(update); err != nil {
			log.Printf("Invalid update message from client: %v", err)
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v", err)))
			continue
		}

		ip := getIP(r)
		if !server.checkRateLimit(ip) {
			conn.WriteMessage(websocket.TextMessage, []byte("rate limit"))
			continue
		}

		server.broadcast <- update
	}
}

func (server *Server) checkRateLimit(ip string) bool {
	now := time.Now()
	if lastUpdate, ok := server.lastUpdate.Load(ip); ok {
		if now.Sub(lastUpdate.(time.Time)) < time.Millisecond*200 {
			return false
		}
	}
	server.lastUpdate.Store(ip, now)
	return true
}

func (server *Server) countClients() int {
	count := 0
	server.clients.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}