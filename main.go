// main.go
// In main.go we wire everything together: upgrade HTTP to WebSocket,
// create a client with a UUID, register it with the manager,
// and spin up the per-connection goroutines. We also start the manager loop and the HTTP server.
// Keep CheckOrigin permissive only for local learning; in production lock it down.

package main

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, validate the origin here.
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}

	client := &Client{id: uuid.NewString(), socket: conn, send: make(chan []byte)}
	manager.register <- client

	go client.read()
	go client.write()
}

func main() {
	fmt.Println("starting server on :12345 ...")
	go manager.start()

	http.HandleFunc("/ws", wsHandler)
	// Optional: serve statis Angular build under "/"
	// http.Handle("/", http.FileServer(http.Dir("./static")))

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic(err)
	}
}
