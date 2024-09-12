package main

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Client wraps a [websocket.Conn] to add thread safety to
// WriteMessage.
type Client struct {
	*websocket.Conn
	m sync.Mutex
}

func NewClient(conn *websocket.Conn) *Client {
	return &Client{
		Conn: conn,
	}
}

func (c *Client) WriteMessage(messageType int, data []byte) error {
	c.m.Lock()
	defer c.m.Unlock()

	return c.Conn.WriteMessage(messageType, data)
}
