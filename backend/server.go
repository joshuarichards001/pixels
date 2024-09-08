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
		broadcast:   make(chan IncomingMessage, 100000),
		register:    make(chan *websocket.Conn, 10000),
		unregister:  make(chan *websocket.Conn, 10000),
		redisClient: rdb,
		ctx:         ctx,
	}
}

func (server *Server) run() {
	go server.handleBroadcasts()
	go server.handleRegistrations()
	go server.handleUnregistrations()
}

func (server *Server) handleBroadcasts() {
	for update := range server.broadcast {
		err := server.redisClient.SetRange(server.ctx, "pixels", int64(update.Data.Index), update.Data.Color).Err()
		if err != nil {
			log.Printf("error updating Redis: %v", err)
			continue
		}

		clientCount := server.countClients()

		msg := OutgoingMessage{Type: "update", Data: update.Data, ClientCount: clientCount}
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

func (server *Server) handleRegistrations() {
	for client := range server.register {
		server.clients.Store(client, true)
	}
}

func (server *Server) handleUnregistrations() {
	for client := range server.unregister {
		server.clients.Delete(client)
		client.Close()
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
	hCaptchaToken := r.Header.Get("Sec-WebSocket-Protocol")
	if err := verifyHCaptcha(hCaptchaToken); err != nil {
		log.Printf("error verifying hCaptcha: %v", err)
		http.Error(w, "could not verify hCaptcha", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, http.Header{"Sec-WebSocket-Protocol": {hCaptchaToken}})
	if err != nil {
		log.Printf("error upgrading connection: %v", err)
		http.Error(w, "could not open websocket connection", http.StatusInternalServerError)
		return
	}

	ip := getIP(r)
	if !server.checkAndUpdateClientCount(ip, true) {
		conn.WriteMessage(websocket.TextMessage, []byte("client limit exceeded"))
		conn.Close()
		return
	}

	server.register <- conn

	defer func() {
		server.unregister <- conn
		server.checkAndUpdateClientCount(ip, false)
		conn.Close()
	}()

	time.AfterFunc(10*time.Minute, func() {
		server.unregister <- conn
		server.checkAndUpdateClientCount(ip, false)
		conn.Close()
	})

	initialData, err := server.redisClient.Get(server.ctx, "pixels").Result()
	if err != nil {
		log.Printf("error getting initial data from Redis: %v", err)
		return
	}

	clientCount := server.countClients()

	initialMsg := InitialMessage{Type: "initial", Data: initialData, ClientCount: clientCount}
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

		if !server.checkRateLimit(ip) {
			conn.WriteMessage(websocket.TextMessage, []byte("rate limit exceeded"))
			continue
		}

		log.Printf("Pixel updated: index=%d, color=%s, ip=%s", update.Data.Index, update.Data.Color, ip)

		server.broadcast <- update
	}
}

func (server *Server) handleGetPixels(w http.ResponseWriter, _ *http.Request) {
	pixelsData, err := server.redisClient.Get(server.ctx, "pixels").Result()
	if err != nil {
		log.Printf("error getting pixels data from Redis: %v", err)
		http.Error(w, "could not retrieve pixels data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(pixelsData))
}

func getAllowedOrigin() string {
	environment := os.Getenv("ENVIRONMENT")
	if environment == "development" {
		return "http://127.0.0.1:5500"
	} else {
		return "https://tenthousandpixels.com"
	}
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", getAllowedOrigin())
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next(w, r)
	}
}

func (server *Server) countClients() int {
	count := 0
	server.clients.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	log.Printf("Client count: %d", count)
	return count
}
