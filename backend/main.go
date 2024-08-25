package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

func validateIncomingMessage(update IncomingMessage) error {
	if update.Type != "update" {
		return fmt.Errorf("invalid message type: %s", update.Type)
	}

	if update.Data.Index < 0 || update.Data.Index >= 10000 {
		return fmt.Errorf("invalid index: %d", update.Data.Index)
	}

	colorNumber, err := strconv.Atoi(update.Data.Color)
	if err != nil {
		return fmt.Errorf("invalid color value: %s", update.Data.Color)
	}

	if colorNumber < 0 || colorNumber > 9 {
		return fmt.Errorf("color value out of range: %d", colorNumber)
	}

	return nil
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
			if err := validateIncomingMessage(update); err != nil {
				log.Printf("Invalid update message: %v", err)
				continue
			}

			server.mu.Lock()
			server.data[update.Data.Index] = []rune(update.Data.Color)[0]
			dataCopy := string(server.data)
			server.mu.Unlock()

			msg := OutgoingMessage{Type: "update", Data: dataCopy}
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

func (server *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		http.Error(w, "Could not open websocket connection", http.StatusInternalServerError)
		return
	}

	server.register <- conn

	defer func() {
		server.unregister <- conn
		conn.Close()
	}()

	server.mu.RLock()
	initialData := string(server.data)
	server.mu.RUnlock()

	initialMsg := OutgoingMessage{Type: "initial", Data: initialData}
	jsonMsg, err := json.Marshal(initialMsg)
	if err != nil {
		log.Printf("Error marshaling initial JSON: %v", err)
		return
	}

	err = conn.WriteMessage(websocket.TextMessage, jsonMsg)
	if err != nil {
		log.Printf("Error sending initial message: %v", err)
		return
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			break
		}

		var update IncomingMessage
		err = json.Unmarshal(msgBytes, &update)
		if err != nil {
			log.Printf("Error unmarshaling JSON: %v", err)
			continue
		}

		server.broadcast <- update
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
