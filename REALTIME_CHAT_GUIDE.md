# Build a Real-Time Chat App with Go (2026) and Angular

Updated for Go 1.21/1.22 and Angular 17+. This guide shows how to build a small but production-minded WebSocket chat: Go handles connections and broadcast; Angular renders the UI.

## What You Will Build
- Go WebSocket server with broadcast + per-connection goroutines
- JSON message envelope with sender/content
- Angular single-page client that streams messages over WebSocket

## Prerequisites
- Go 1.21+ (1.22 recommended)
- Node.js 18+ (20+ recommended) and npm
- Angular CLI 17+ (`npm install -g @angular/cli`)
- A terminal and a browser (Chrome/Edge/Firefox)

## Backend: Go WebSocket Server

### 1) Init module and dependencies
Create a fresh folder, initialize a Go module, then add the two libraries we need. Gorilla WebSocket upgrades HTTP to WS and frames messages; google/uuid gives us stable unique ids for each client connection.
```bash
go mod init example.com/go-chat
# Gorilla WebSocket for upgrades and framing
go get github.com/gorilla/websocket
# UUIDs for client ids
go get github.com/google/uuid
```

### 2) Core types
Create a new file client_manager.go that defines the core structs the server will share. The `ClientManager` holds all connections plus the channels we will use to coordinate registration and broadcast. Each `Client` wraps a WebSocket plus a send channel, and `Message` is our JSON envelope so the front-end knows who sent what.
```go
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
```

### 3) Server event loop
Create another file manager.go for the central event loop. This loop is the traffic cop: it registers and unregisters clients and fans out any incoming broadcast messages to every live client.
```go
// manager.go
package main

import "encoding/json"

var manager = ClientManager{
    broadcast:  make(chan []byte),
    register:   make(chan *Client),
    unregister: make(chan *Client),
    clients:    make(map[*Client]bool),
}

func (m *ClientManager) start() {
    for {
        select {
        case conn := <-m.register:
            m.clients[conn] = true
            msg, _ := json.Marshal(&Message{Content: "/A new socket connected."})
            m.send(msg, conn)
        case conn := <-m.unregister:
            if _, ok := m.clients[conn]; ok {
                close(conn.send)
                delete(m.clients, conn)
                msg, _ := json.Marshal(&Message{Content: "/A socket disconnected."})
                m.send(msg, conn)
            }
        case message := <-m.broadcast:
            for conn := range m.clients {
                select {
                case conn.send <- message:
                default:
                    close(conn.send)
                    delete(m.clients, conn)
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
```

### 4) Per-connection read/write goroutines
Add client.go to hold the logic that runs per connection. The `read` goroutine listens to messages from the browser and pushes them into `manager.broadcast`. The `write` goroutine drains the client’s `send` channel back to the browser. Separating read/write avoids head-of-line blocking when a browser is slow.
```go
// client.go
package main

import (
    "encoding/json"
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

    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                c.socket.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            c.socket.WriteMessage(websocket.TextMessage, message)
        }
    }
}
```

### 5) HTTP upgrade handler and main
In main.go we wire everything together: upgrade HTTP to WebSocket, create a client with a UUID, register it with the manager, and spin up the per-connection goroutines. We also start the manager loop and the HTTP server. Keep `CheckOrigin` permissive only for local learning; in production lock it down.
```go
// main.go
package main

import (
    "fmt"
    "net/http"

    "github.com/google/uuid"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true }, // relax for demo; tighten for prod
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
    fmt.Println("Starting chat server on :12345 ...")
    go manager.start()

    http.HandleFunc("/ws", wsHandler)
    // Optional: serve static Angular build under /
    // http.Handle("/", http.FileServer(http.Dir("./ui/dist")))

    if err := http.ListenAndServe(":12345", nil); err != nil {
        panic(err)
    }
}
```

### 6) Run the server
From the folder where your Go files live, start the server. It listens on port 12345 and exposes one WebSocket endpoint at /ws.
```bash
go run .
# WebSocket endpoint: ws://localhost:12345/ws
```

## Frontend: Angular Client

### 1) Create the project
Generate a new Angular app. Angular CLI scaffolds the project structure; Material is optional but useful for styling later.
```bash
ng new websocket-chat --style=css --routing=false
cd websocket-chat
ng add @angular/material # optional but handy
```

### 2) Socket service
Create src/app/socket.service.ts. This service owns one WebSocket instance and exposes an RxJS stream so components can subscribe to incoming messages. It also wraps basic lifecycle events (open/close/error) as system messages for the UI.
```ts
// src/app/socket.service.ts
import { Injectable } from '@angular/core';
import { Observable, Subject } from 'rxjs';

export interface ChatMessage {
  sender?: string;
  content?: string;
}

@Injectable({ providedIn: 'root' })
export class SocketService {
  private socket?: WebSocket;
  private stream$ = new Subject<ChatMessage>();

  connect(url = 'ws://localhost:12345/ws'): Observable<ChatMessage> {
    if (this.socket) return this.stream$.asObservable();

    this.socket = new WebSocket(url);
    this.socket.onopen = () => this.stream$.next({ content: '/Connected' });
    this.socket.onclose = () => this.stream$.next({ content: '/Disconnected' });
    this.socket.onmessage = (event) => this.stream$.next(JSON.parse(event.data));
    this.socket.onerror = () => this.stream$.next({ content: '/Error' });

    return this.stream$.asObservable();
  }

  send(text: string) {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) return;
    this.socket.send(text);
  }

  close() {
    this.socket?.close();
  }
}
```

### 3) Component logic
Edit src/app/app.component.ts. Here we subscribe to the socket stream, keep a list of messages to render, and provide a `send` method bound to the input form. The helper `render` adds a label for system messages and prefixes user messages with the sender id when present.
```ts
// src/app/app.component.ts
import { Component, OnDestroy, OnInit } from '@angular/core';
import { Subscription } from 'rxjs';
import { ChatMessage, SocketService } from './socket.service';

@Component({
  selector: 'app-root',
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css'],
})
export class AppComponent implements OnInit, OnDestroy {
  messages: ChatMessage[] = [];
  chatBox = '';
  private sub?: Subscription;

  constructor(private socket: SocketService) {}

  ngOnInit(): void {
    this.sub = this.socket.connect().subscribe((msg) => this.messages.push(msg));
  }

  ngOnDestroy(): void {
    this.sub?.unsubscribe();
    this.socket.close();
  }

  send() {
    if (!this.chatBox.trim()) return;
    this.socket.send(this.chatBox);
    this.chatBox = '';
  }

  render(msg: ChatMessage): string {
    if ((msg.content || '').startsWith('/')) return `<strong>${(msg.content || '').slice(1)}</strong>`;
    const sender = msg.sender ? `${msg.sender}: ` : '';
    return sender + (msg.content || '');
  }
}
```

### 4) Template and styles
Add the HTML and CSS for the chat surface. The template loops over messages and binds the form to `chatBox`. The CSS makes the layout full-height with a fixed footer form so the input stays accessible.
```html
<!-- src/app/app.component.html -->
<div class="chat">
  <ul id="messages">
    <li *ngFor="let m of messages" [innerHTML]="render(m)"></li>
  </ul>
  <form (submit)="send(); $event.preventDefault();">
    <input [(ngModel)]="chatBox" name="chat" autocomplete="off" />
    <button type="submit">Send</button>
  </form>
</div>
```

```css
/* src/app/app.component.css */
* { box-sizing: border-box; }
body, html { margin: 0; font: 14px Helvetica, Arial, sans-serif; }
.chat { display: flex; flex-direction: column; height: 100vh; }
#messages { flex: 1; overflow-y: auto; list-style: none; margin: 0; padding: 0; }
#messages li { padding: 6px 10px; }
#messages li:nth-child(odd) { background: #f2f2f2; }
form { display: flex; padding: 8px; background: #111; gap: 8px; }
form input { flex: 1; padding: 10px; border: none; border-radius: 4px; }
form button { padding: 10px 16px; border: none; border-radius: 4px; background: #3fb5ff; color: #000; font-weight: 600; cursor: pointer; }
form button:hover { background: #2f9edd; }
```

### 5) Run the UI
Start the Angular dev server. It serves the UI at http://localhost:4200 and connects to the Go WebSocket endpoint on :12345.
```bash
ng serve --open
# App runs at http://localhost:4200 and talks to ws://localhost:12345/ws
```

## Hardening for Production (quick hits)
- **CORS/Origin**: Lock `CheckOrigin` to your domains instead of returning `true`.
- **TLS**: Terminate HTTPS/WSS with a reverse proxy (Nginx, Caddy) or use `ListenAndServeTLS`.
- **Auth**: Issue JWT or session cookies; include user id in the message envelope; reject unauthenticated upgrades.
- **Backpressure**: Consider bounded channel sizes per client; drop or disconnect slow consumers.
- **Rooms/Topics**: Add a `room` field in `Message` and map room membership to separate broadcast channels.
- **Persistence**: Store history in Postgres/Redis; stream the latest N messages on connect.
- **Scaling**: Fan-out via Redis pub/sub or NATS between multiple Go instances.

## Quick Troubleshooting
- **WebSocket 400/upgrade fails**: Browser must use `ws://localhost:12345/ws`; if you serve Angular over HTTPS, use `wss://` and TLS on the Go side.
- **No messages shown**: Check server logs; verify the Go process is running and `broadcast` is reached.
- **CORS errors**: Tighten `CheckOrigin` to expected hosts, or configure your proxy to set correct `Origin`/`Host`.

You now have a modern Go + Angular WebSocket chat foundation you can adapt for IoT, gaming lobbies, or support dashboards.
