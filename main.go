package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	// "os"
	
	"github.com/gorilla/websocket"
)

type M map[string]interface{}

const MESSAGE_NEW_USER = "New User"
const MESSAGE_CHAT = "Chat"
const MESSAGE_LEAVE = "Leave"

var connections = make([]*WebSocketConnection, 0)
var rooms = make(map[string]*Room)

type SocketPayload struct {
	Message string
}

type SocketResponse struct {
	From    string
	Type    string
	Message string
}

type WebSocketConnection struct {
	*websocket.Conn
	Username string
	Room     string
}

type Room struct {
	Name              string
	Connections       []*WebSocketConnection
	DirectConnections []*WebSocketConnection
}

func main() {

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		currentGorillaConn, err := websocket.Upgrade(w, r, w.Header(), 1024, 1024)
		if err != nil {
			http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
			return
		}
	
		username := r.URL.Query().Get("username")
		room := r.URL.Query().Get("room") // Get the room name from the query parameter
		currentConn := WebSocketConnection{Conn: currentGorillaConn, Username: username, Room: room}
		connections = append(connections, &currentConn)
	
		// Create the room if it doesn't exist
		if _, ok := rooms[room]; !ok {
			rooms[room] = &Room{Name: room, Connections: []*WebSocketConnection{}}
		}
		rooms[room].Connections = append(rooms[room].Connections, &currentConn)
	
		go handleIO(&currentConn, connections)
	})

	fmt.Println("Server starting at :8181")
	http.ListenAndServe(":8181", nil)
}

func handleIO(currentConn *WebSocketConnection, connections []*WebSocketConnection) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("ERROR", fmt.Sprintf("%v", r))
		}
	}()

	broadcastMessage(currentConn, MESSAGE_NEW_USER, "")

	for {
		payload := SocketPayload{}
		err := currentConn.ReadJSON(&payload)
		if err != nil {
			if strings.Contains(err.Error(), "websocket: close") {
				broadcastMessage(currentConn, MESSAGE_LEAVE, "")
				ejectConnection(currentConn)
				return
			}

			log.Println("ERROR", err.Error())
			continue
		}

		if strings.HasPrefix(payload.Message, "/direct") {
			// Mengirim pesan langsung ke pengguna lain
			recipient := strings.TrimPrefix(payload.Message, "/direct ")
			sendDirectMessage(currentConn, recipient, strings.TrimSpace(recipient))
		} else {
			// Mengirim pesan broadcast ke semua pengguna dalam ruangan
			broadcastMessage(currentConn, MESSAGE_CHAT, payload.Message)
		}
	}
}

func sendDirectMessage(sender *WebSocketConnection, recipient, message string) {
	room := rooms[sender.Room]
	if room == nil {
		return
	}

	for _, eachConn := range room.Connections {
		if eachConn.Username == recipient {
			eachConn.WriteJSON(SocketResponse{
				From:    sender.Username,
				Type:    MESSAGE_CHAT,
				Message: message,
			})
			return
		}
	}

	// Jika pengguna penerima tidak ditemukan
	sender.WriteJSON(SocketResponse{
		From:    "System",
		Type:    MESSAGE_CHAT,
		Message: "User not found",
	})
}

func ejectConnection(currentConn *WebSocketConnection) {
	var filtered []*WebSocketConnection
	for _, conn := range connections {
		if conn != currentConn {
			filtered = append(filtered, conn)
		}
	}
	connections = filtered
}

func broadcastMessage(currentConn *WebSocketConnection, kind, message string) {
	room := rooms[currentConn.Room]
	if room == nil {
		return
	}

	for _, eachConn := range room.Connections {
		if eachConn == currentConn {
			continue
		}

		eachConn.WriteJSON(SocketResponse{
			From:    currentConn.Username,
			Type:    kind,
			Message: message,
		})
	}
}
