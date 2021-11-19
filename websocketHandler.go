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

type ClientManager struct {
	clients       map[*Client]bool
	broadcast     chan []byte
	register      chan *Client
	unregister    chan *Client
	broadcastTime chan []byte
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

type Pack struct {
	Msg  string `json:"msg"`
	Time string `json:"time"`
}

var timeTable []int

func (manager *ClientManager) start() {

	for {
		select {
		case conn := <-manager.register:
			manager.clients[conn] = true
			//jsonMsg, _ := json.Marshal(&Message{Content: "a new socket has connect"})
			//manager.send(jsonMsg, conn)

		case conn := <-manager.unregister:
			if _, ok := manager.clients[conn]; ok {
				close(conn.send)
				delete(manager.clients, conn)
				//jsonMsg, _ := json.Marshal(&Message{Content: "a socket has disconnect"})
				//manager.send(jsonMsg, conn)
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
		//fmt.Println(string(msg))
		var webmsg Pack
		json.Unmarshal(msg, &webmsg)
		//fmt.Println(webmsg)

		if webmsg.Msg != "" && webmsg.Time == "" { //聊天訊息
			jsonMsg, _ := json.Marshal(&Message{Sender: c.id, Content: string(webmsg.Msg)})
			manager.broadcast <- jsonMsg
		} else {
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
	//fmt.Println(u.Hostname())
	//fmt.Println(u.Path)
	//fmt.Println(u.RawQuery)
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
