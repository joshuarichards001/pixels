package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins
}

type Server struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan string
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	data       string
	mu         sync.Mutex
}

type Message struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func newServer() *Server {
	return &Server{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan string),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		data:       strings.Repeat("0", 10000),
	}
}

func (s *Server) run() {
	for {
		select {
		case client := <-s.register:
			s.clients[client] = true
		case client := <-s.unregister:
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				client.Close()
			}
		case message := <-s.broadcast:
			s.mu.Lock()
			s.data = message
			s.mu.Unlock()
			for client := range s.clients {
				msg := Message{Type: "update", Data: message}
				jsonMsg, err := json.Marshal(msg)
				if err != nil {
					log.Printf("error marshaling json: %v", err)
					continue
				}
				err = client.WriteMessage(websocket.TextMessage, jsonMsg)
				if err != nil {
					log.Printf("error: %v", err)
					client.Close()
					delete(s.clients, client)
				}
			}
		}
	}
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	defer conn.Close()

	s.register <- conn

	s.mu.Lock()
	initialData := s.data
	s.mu.Unlock()

	initialMsg := Message{Type: "initial", Data: initialData}
	jsonMsg, err := json.Marshal(initialMsg)
	if err != nil {
		log.Printf("error marshaling json: %v", err)
		return
	}
	err = conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			s.unregister <- conn
			break
		}
		var msg Message
		err = json.Unmarshal(msgBytes, &msg)
		if err != nil {
			log.Printf("error unmarshaling json: %v", err)
			continue
		}
		if msg.Type == "update" {
			s.broadcast <- msg.Data
		}
	}
}

func main() {
	server := newServer()
	go server.run()

	http.HandleFunc("/ws", server.handleConnections)

	fmt.Println("Server is running on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}