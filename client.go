package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Max wait time when writing message to peer
	writeWait = 10 * time.Second

	// Max time till next pong from peer
	pongWait = 60 * time.Second

	// Send ping interval, must be less then pong wait time
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 10000
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client represents the websocket client at the server
type Client struct {
	// The actual websocket connection.
	conn     *websocket.Conn
	wsServer *WsServer
	send     chan []byte
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	channels map[*Channel]bool
}

func newClient(conn *websocket.Conn, wsServer *WsServer, name string) *Client {
	return &Client{
		ID:       uuid.New(),
		Name:     name,
		conn:     conn,
		wsServer: wsServer,
		send:     make(chan []byte, 256),
		channels: make(map[*Channel]bool),
	}
}

func (client *Client) readPump() {
	defer func() {
		client.disconnect()
	}()

	client.conn.SetReadLimit(maxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error { client.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	// Start endless read loop, waiting for messages from client
	for {
		_, jsonMessage, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected close error: %v", err)
			}
			break
		}

		client.handleNewMessage(jsonMessage)
	}

}

func (client *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The WsServer closed the channel.
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Attach queued chat messages to the current websocket message.
			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (client *Client) disconnect() {
	client.wsServer.unsubscribe <- client
	for channel := range client.channels {
		channel.unsubscribe <- client
	}
	close(client.send)
	client.conn.Close()
}

// ServeWs handles websocket requests from clients requests.
func serveWs(wsServer *WsServer, w http.ResponseWriter, r *http.Request) {

	name, ok := r.URL.Query()["name"]

	if !ok || len(name[0]) < 1 {
		log.Println("Url Param 'name' is missing")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return
	}

	client := newClient(conn, wsServer, name[0])

	go client.writePump()
	go client.readPump()

	wsServer.subscribe <- client
}

func (client *Client) handleNewMessage(jsonMessage []byte) {

	var message Message
	if err := json.Unmarshal(jsonMessage, &message); err != nil {
		log.Printf("Error on unmarshal JSON message %s", err)
		return
	}

	message.Sender = client

	switch message.Action {
	case SendMessageAction:
		channelID := message.Target.GetId()
		if channel := client.wsServer.findChannelByID(channelID); channel != nil {
			channel.broadcast <- &message
		}

	case JoinChannelAction:
		client.handleJoinChannelMessage(message)

	case LeaveChannelAction:
		client.handleLeaveChannelMessage(message)

	case JoinChannelPrivateAction:
		client.handleJoinChannelPrivateMessage(message)
	}

}

func (client *Client) handleJoinChannelMessage(message Message) {
	channelName := message.Message

	client.joinChannel(channelName, nil)
}

func (client *Client) handleLeaveChannelMessage(message Message) {
	channel := client.wsServer.findChannelByID(message.Message)
	if channel == nil {
		return
	}

	if _, ok := client.channels[channel]; ok {
		delete(client.channels, channel)
	}

	channel.unsubscribe <- client
}

func (client *Client) handleJoinChannelPrivateMessage(message Message) {

	target := client.wsServer.findClientByID(message.Message)

	if target == nil {
		return
	}

	// create unique channel name combined to the two IDs
	channelName := message.Message + client.ID.String()

	client.joinChannel(channelName, target)
	target.joinChannel(channelName, client)

}

func (client *Client) joinChannel(channelName string, sender *Client) {

	channel := client.wsServer.findChannelByName(channelName)

	if channel == nil {
		channel = client.wsServer.createChannel(channelName, sender != nil)
	}

	// Don't allow to join private channels through public channel message
	if sender == nil && channel.Private {
		return
	}

	if !client.isInChannel(channel) {

		client.channels[channel] = true
		channel.subscribe <- client

		client.notifyChannelJoined(channel, sender)
	}
}

func (client *Client) isInChannel(channel *Channel) bool {
	if _, ok := client.channels[channel]; ok {
		return true
	}

	return false
}

func (client *Client) notifyChannelJoined(channel *Channel, sender *Client) {
	message := Message{
		Action: ChannelJoinedAction,
		Target: channel,
		Sender: sender,
	}

	client.send <- message.encode()
}

func (client *Client) GetName() string {
	return client.Name
}
