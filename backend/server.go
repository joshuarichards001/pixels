package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"
)

type Server struct {
	broadcast  chan broadcast[IncomingMessage]
	register   chan registration
	unregister chan *Client
	numclients chan int

	redisClient *redis.Client
}

type registration struct {
	client *Client
	rsp    chan error
}

func NewServer() *Server {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDRESS"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	return &Server{
		broadcast:   make(chan broadcast[IncomingMessage]),
		register:    make(chan registration),
		unregister:  make(chan *Client),
		numclients:  make(chan int),
		redisClient: rdb,
	}
}

func (server *Server) Run(ctx context.Context) {
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

		case update := <-server.broadcast:
			if !rateLimits[update.src.Addr].limiter.Allow() {
				update.src.WriteMessage(websocket.TextMessage, []byte("rate limit exceeded"))
				continue
			}

			log.Printf("Pixel updated: index=%d, color=%s, ip=%s", update.msg.Data.Index, update.msg.Data.Color, update.src.Addr)

			err := server.redisClient.SetRange(ctx, "pixels", int64(update.msg.Data.Index), update.msg.Data.Color).Err()
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

		case r := <-server.register:
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

		case client := <-server.unregister:
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

		case server.numclients <- len(clients):
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

	regrsp := make(chan error)
	server.register <- registration{client: &client, rsp: regrsp}
	err = <-regrsp
	if err != nil {
		client.WriteMessage(websocket.TextMessage, []byte("client limit exceeded"))
		client.Close()
		return
	}
	defer func() {
		server.unregister <- &client
	}()

	initialData, err := server.redisClient.Get(req.Context(), "pixels").Result()
	if err != nil {
		log.Printf("error getting initial data from Redis: %v", err)
		return
	}

	initialMsg := InitialMessage{Type: "initial", Data: initialData, ClientCount: <-server.numclients}
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

		server.broadcast <- broadcast[IncomingMessage]{
			src: &client,
			msg: update,
		}
	}
}

type broadcast[T any] struct {
	src *Client
	msg T
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
