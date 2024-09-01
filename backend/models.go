package main

import (
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
)

type Server struct {
	clients     sync.Map
	broadcast   chan IncomingMessage
	register    chan *websocket.Conn
	unregister  chan *websocket.Conn
	redisClient *redis.Client
	ctx         context.Context
	lastUpdate  sync.Map
}

type OutgoingMessage struct {
	Type        string `json:"type"`
	Data        string `json:"data"`
	ClientCount int    `json:"clientCount"`
}

type IncomingMessage struct {
	Type string       `json:"type"`
	Data UpdatedColor `json:"data"`
}

type UpdatedColor struct {
	Index int    `json:"index"`
	Color string `json:"color"`
}
