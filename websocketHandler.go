package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

type ClientManager struct { //save all client's message
	clients       map[*Client]bool // online : true, offline : false
	broadcast     chan []byte      // save comments from web(clients)
	register      chan *Client
	unregister    chan *Client
	broadcastTime chan []byte // save video time from web(clients)
}

type Client struct {
	id     string
	socket *websocket.Conn
	send   chan []byte // send message from web send frame
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

type Pack struct {
	Msg  string `json:"msg"`
	Time string `json:"time"`
}

var timeTable []int // save all clients video time

func (manager *ClientManager) start() { // listen register & unregister and broadcast channel

	for {
		select {
		case conn := <-manager.register: // if someone enter the url
			manager.clients[conn] = true
			/*Test*/
			//jsonMsg, _ := json.Marshal(&Message{Content: "a new socket has connect"})
			//manager.send(jsonMsg, conn)

		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				/*Test*/
				//jsonMsg, _ := json.Marshal(&Message{Content: "a socket has disconnect"})
				//manager.send(jsonMsg, conn)
			}

		case message := <-manager.broadcast:
			// broadcast message to client send channel by iterate
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

//func (manager *ClientManager) send(message []byte, ignore *Client) {
//	for conn := range manager.clients {
//		if conn != ignore {
//			conn.send <- message
//		}
//	}
//}

// read message from web, send the message to broadcast channel
func (c *Client) read() {
	defer func() {
		manager.unregister <- c
		c.socket.Close()
	}()
	for {
		//read message from web client
		_, msg, err := c.socket.ReadMessage()
		if err != nil {
			manager.unregister <- c
			c.socket.Close()
			break
		}
		var webmsg Pack // video time & comments message
		json.Unmarshal(msg, &webmsg)

		if webmsg.Msg != "" && webmsg.Time == "" { // comments message
			jsonMsg, _ := json.Marshal(&Message{Sender: c.id, Content: string(webmsg.Msg)})
			manager.broadcast <- jsonMsg
		} else { // video time message
			Itime, _ := strconv.Atoi(webmsg.Time)
			timeTable = append(timeTable, Itime)

			if len(timeTable) >= len(manager.clients) {
				var minV = timeTable[0]
				for _, element := range timeTable {
					if element < minV {
						minV = element
					}
				}
				//fmt.Println(minV) // test
				timeTable = nil
				jsonMsg, _ := json.Marshal(&Message{Sender: "time", Content: strconv.Itoa(minV)})
				manager.broadcast <- jsonMsg
			}
		}
	}
}
func (c *Client) write() { //read client send channel(from broadcast channel in start() func）then pass to web client
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

	go client.read()  // receive message from web
	go client.write() // send message to web (from broadcast channel to client own send channel)
}
func socketPlayHandler(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("webclient.html")
	if err != nil {
		fmt.Println("Could not open file.", err)
	}
	fmt.Fprintf(w, "%s", content)

}

func home(w http.ResponseWriter, r *http.Request) {
	var tmpl = template.Must(template.ParseFiles("./index.html"))

	tmpl.Execute(w, struct {
		Title string
	}{
		"SYNCTUBE",
	})

}
func check(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Fatal("ParseForm: ", err)
	}
	yturl := r.Form["url"][0]
	u, err := url.Parse(yturl)

	if u.Hostname() != "www.youtube.com" { //404
		return
	}

	uuid := uuid.Must(uuid.NewV4(), nil).String()
	http.Redirect(w, r, "socket/"+uuid+"?"+u.RawQuery, 302)
}

func main() {
	fmt.Println("starting ....")
	go manager.start()

	m := mux.NewRouter()
	m.HandleFunc("/socket", socketHandler)
	m.HandleFunc("/socket/{code}", socketPlayHandler)
	m.HandleFunc("/", home)
	m.HandleFunc("/check", check)
	http.Handle("/", m)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
