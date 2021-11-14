package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"log"
	"net/http"
)

type ClientManager struct {
	//客户端 map 储存并管理所有的长连接client，在线的为true，不在的为false
	clients map[*Client]bool
	//web端发送来的的message我们用broadcast来接收，并最后分发给所有的client
	broadcast chan []byte
	//新创建的长连接client
	register chan *Client
	//新注销的长连接client
	unregister chan *Client
}

type Client struct {
	id     string
	socket *websocket.Conn
	send   chan []byte
}

type Message struct {
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Content   string `json:"content,omitempty"`
}

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
			jsonMsg, _ := json.Marshal(&Message{Content: "a new socket has connect"})
			manager.send(jsonMsg, conn)

		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				jsonMsg, _ := json.Marshal(&Message{Content: "a socket has disconnect"})
				manager.send(jsonMsg, conn)
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

func (manager *ClientManager) send(message []byte, ignore *Client) {
	for conn := range manager.clients {
		if conn != ignore {
			conn.send <- message
		}
	}
}
func (c *Client) read() { //讀取從web端輸入的message，並把message 傳給broadcast使得其能夠廣播給其他client
	defer func() {
		manager.unregister <- c
		c.socket.Close()
	}()
	for {
		//read msg from web client
		_, msg, err := c.socket.ReadMessage()
		if err != nil {
			manager.unregister <- c
			c.socket.Close()
			break
		}
		jsonMsg, _ := json.Marshal(&Message{Sender: c.id, Content: string(msg)})
		manager.broadcast <- jsonMsg
	}
}
func (c *Client) write() { //讀取client send channel 的訊息（從broadcast channel得到的訊息）傳送給web client端
	defer func() {
		c.socket.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				c.socket.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.socket.WriteMessage(websocket.TextMessage, msg)
		}
	}
}

func socketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade our raw HTTP connection to a websocket based one
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}).Upgrade(w, r, nil)
	if err != nil {
		log.Print("Error during connection upgradation:", err)
		return
	}

	client := &Client{id: uuid.Must(uuid.NewV4(), nil).String(), socket: conn, send: make(chan []byte)}
	manager.register <- client

	go client.read()
	go client.write()
}

//func home(w http.ResponseWriter, r *http.Request) {
//	fmt.Fprintf(w, "Index Page")
//}

func main() {
	fmt.Println("starting ....")
	go manager.start()

	http.HandleFunc("/socket", socketHandler)
	//http.HandleFunc("/", home)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
