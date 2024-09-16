package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

type Server struct {
	clients     *clientManager
	redisClient *redis.Client
}

func NewServer() *Server {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDRESS"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	return &Server{
		clients:     newClientManager(rdb),
		redisClient: rdb,
	}
}

func (server *Server) Run(ctx context.Context) {
	server.clients.Run(ctx)
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

func (server *Server) handleConnections(rw http.ResponseWriter, req *http.Request) {
	hCaptchaToken := req.Header.Get("Sec-WebSocket-Protocol")
	if err := verifyHCaptcha(hCaptchaToken); err != nil {
		log.Printf("error verifying hCaptcha: %v", err)
		http.Error(rw, "could not verify hCaptcha", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(rw, req, http.Header{"Sec-WebSocket-Protocol": {hCaptchaToken}})
	if err != nil {
		log.Printf("error upgrading connection: %v", err)
		http.Error(rw, "could not open websocket connection", http.StatusInternalServerError)
		return
	}
	addr, ok := getIP(req, conn)
	if !ok {
		log.Printf("could not determine identifiable IP address for connection from %v", req.RemoteAddr)
		http.Error(rw, "could not determine necessary information", http.StatusInternalServerError)
		return
	}
	client := Client{
		Conn: conn,
		Addr: addr,
	}

	err = server.clients.Register(&client)
	if err != nil {
		client.WriteMessage(websocket.TextMessage, []byte("client limit exceeded"))
		client.Close()
		return
	}
	defer server.clients.Unregister(&client)

	initialData, err := server.redisClient.Get(req.Context(), "pixels").Result()
	if err != nil {
		log.Printf("error getting initial data from Redis: %v", err)
		return
	}

	initialMsg := InitialMessage{Type: "initial", Data: initialData, ClientCount: server.clients.Num()}
	jsonMsg, err := json.Marshal(initialMsg)
	if err != nil {
		log.Printf("error marshaling initial JSON: %v", err)
		return
	}

	err = client.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("error sending initial message: %v", err)
		return
	}

	for {
		_, msgBytes, err := client.ReadMessage()
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
			client.WriteMessage(websocket.TextMessage, []byte("Invalid input type"))
			continue
		}

		if err := validateIncomingMessage(update); err != nil {
			log.Printf("Invalid update message from client: %v", err)
			client.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: %v", err)))
			continue
		}

		server.clients.Broadcast(&client, update)
	}
}

func (server *Server) handleGetPixels(rw http.ResponseWriter, req *http.Request) {
	pixelsData, err := server.redisClient.Get(req.Context(), "pixels").Result()
	if err != nil {
		log.Printf("error getting pixels data from Redis: %v", err)
		http.Error(rw, "could not retrieve pixels data", http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(pixelsData))
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
