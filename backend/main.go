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
	ReadBufferSize:  16384,
	WriteBufferSize: 512,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins
}

type Server struct {
	clients    sync.Map
	broadcast  chan IncomingMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	data       []rune
	mu         sync.RWMutex
}

type OutgoingMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type IncomingMessage struct {
	Type string       `json:"type"`
	Data UpdatedColor `json:"data"`
}

type UpdatedColor struct {
	Index int    `json:"index"`
	Color string `json:"color"`
}

func newServer() *Server {
	return &Server{
		broadcast:  make(chan IncomingMessage),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		data:       []rune(strings.Repeat("0", 10000)),
	}
}

func (s *Server) run() {
	for {
		select {
		case client := <-s.register:
			s.clients.Store(client, true)
		case client := <-s.unregister:
			s.clients.Delete(client)
			client.Close()
		case update := <-s.broadcast:
			s.mu.Lock()
			if update.Data.Index >= 0 && update.Data.Index < 10000 {
				s.data[update.Data.Index] = []rune(update.Data.Color)[0]
			}
			dataCopy := string(s.data)
			s.mu.Unlock()

			s.clients.Range(func(key, value interface{}) bool {
				client := key.(*websocket.Conn)
				msg := OutgoingMessage{Type: "update", Data: dataCopy}
				jsonMsg, err := json.Marshal(msg)
				if err != nil {
					log.Printf("error marshaling json: %v", err)
					return true
				}
				err = client.WriteMessage(websocket.TextMessage, jsonMsg)
				if err != nil {
					log.Printf("error: %v", err)
					client.Close()
					s.clients.Delete(client)
				}
				return true
			})
		}
	}
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}

	s.register <- conn

	s.mu.RLock()
	initialData := string(s.data)
	s.mu.RUnlock()

	initialMsg := OutgoingMessage{Type: "initial", Data: initialData}
	jsonMsg, err := json.Marshal(initialMsg)
	if err != nil {
		log.Printf("error marshaling json: %v", err)
		conn.Close()
		return
	}
	err = conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("error: %v", err)
		conn.Close()
		return
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			s.unregister <- conn
			break
		}
		var update IncomingMessage
		err = json.Unmarshal(msgBytes, &update)
		if err != nil {
			log.Printf("error unmarshaling json: %v", err)
			continue
		}
		s.broadcast <- update
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
