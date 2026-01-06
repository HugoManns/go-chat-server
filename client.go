// Client.go
// The read goroutine listens to messages from the browser and pushes them into manager.broadcast.
// The write goroutine drains the client’s send channel back to the browser.
// Separating read/write avoids head-of-line blocking when a browser is slow.

package main

import (
	"encoding/json"

	"github.com/gorilla/websocket"
)

func (c *Client) read() {
	defer func() {
		manager.unregister <- c
		c.socket.Close()
	}()

	for {
		_, message, err := c.socket.ReadMessage()
		if err != nil {
			manager.unregister <- c
			c.socket.Close()
			break
		}
		jsonMessage, _ := json.Marshal(&Message{Sender: c.id, Content: string(message)})
		manager.broadcast <- jsonMessage

	}
}

func (c *Client) write() {
	defer c.socket.Close()

	for message := range c.send {
		c.socket.WriteMessage(websocket.TextMessage, message)
	}
	c.socket.WriteMessage(websocket.CloseMessage, []byte{})
}
