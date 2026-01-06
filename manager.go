// manager.go

// central event loop. The manager handles client registration, unregistration, and message broadcasting.
package main

import "encoding/json"

var manager = ClientManager{
	broadcast:  make(chan []byte),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]bool),
}

func (manager *ClientManager) start() {
	for {
		select {
		case conn := <-manager.register:
			manager.clients[conn] = true
			msg, _ := json.Marshal(Message{Content: "New client connected"})
			manager.send(msg, conn)

		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				msg, _ := json.Marshal(&Message{Content: "/A socket disconnected."})
				manager.send(msg, conn)
			}
		case message := <-manager.broadcast:
			for conn := range manager.clients {
				select {
				case conn.send <- message:
				default:
					close(conn.send)
					delete(manager.clients, conn)
				}
			}
		}
	}
}

// send broadcasts to all except the ignored client.
func (m *ClientManager) send(message []byte, ignore *Client) {
	for conn := range m.clients {
		if conn != ignore {
			conn.send <- message
		}
	}
}
