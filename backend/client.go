package main

import (
	"net/netip"
	"sync"

	"github.com/gorilla/websocket"
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
