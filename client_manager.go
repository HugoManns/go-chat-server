// client_manager.go
package main

import "github.com/gorilla/websocket"

// ClientManager tracks connected clients and broadcast traffic.
type ClientManager struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}

// Client represents a single WebSocket connection.
type Client struct {
    id     string
    socket *websocket.Conn
    send   chan []byte
}

// Message is the JSON payload exchanged between server and UI.
type Message struct {
    Sender    string `json:"sender,omitempty"`
    Recipient string `json:"recipient,omitempty"`
    Content   string `json:"content,omitempty"`
}