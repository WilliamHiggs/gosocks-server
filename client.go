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
	plus = []byte{'+'}
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
	channels map[*Channel]bool
}

func newClient(conn *websocket.Conn, wsServer *WsServer) *Client {
	return &Client{
		ID:       uuid.New(),
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
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
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
				log.Printf("write-pump client closed the channel")
				return
			}

			w, err := client.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("write-pump error on next writer %s", err)
				return
			}
			w.Write(message)

			// Attach queued chat messages to the current websocket message.
			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write(plus)
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				log.Printf("write-pump error on close %s", err)
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("write-pump error on write %s", err)
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

	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Println(err)
		return
	}

	client := newClient(conn, wsServer)

	go client.writePump()
	go client.readPump()

	wsServer.subscribe <- client
}

func (client *Client) handleNewMessage(jsonMessage []byte) {

	var message Message

	if err := json.Unmarshal(jsonMessage, &message); err != nil {
		log.Printf("Error on unmarshal JSON message %s", err)
		webhook(client.ID.String(), ChannelUnexpectedError)
		return
	}

	message.Sender = client

	log.Printf("handleNewMessage: %v", message)

	switch message.Action {

	case SendMessageAction:
		webhook(client.ID.String(), SendMessageAction)
		client.handleSendMessage(&message)

	case JoinChannelAction:
		webhook(client.ID.String(), JoinChannelAction)
		client.joinChannel(message)

	case LeaveChannelAction:
		webhook(client.ID.String(), LeaveChannelAction)
		client.handleLeaveChannelMessage(message)

	case JoinChannelPrivateAction:
		webhook(client.ID.String(), JoinChannelPrivateAction)
		message.Sender = nil
		client.joinChannel(message)
	default:
		log.Printf("Unknown action %s", message.Action)
	}
}

func (client *Client) handleSendMessage(message *Message) {
	if channel := client.wsServer.findChannelByName(message.Name); channel != nil {
		if !channel.Private {
			log.Printf("Blocked sending message from a non private channel %s", channel.Name)
			return
		}

		message.Target = channel
		message.Timestamp = time.Now().Unix()

		channel.broadcast <- message
	}
}

func (client *Client) handleLeaveChannelMessage(message Message) {
	channel := client.wsServer.findChannelByName(message.Name)

	if channel == nil {
		log.Printf("Tried to leave a channel that doesn't exist %s", message.Name)
		return
	}

	if client.isInChannel(channel) {
		delete(client.channels, channel)
	}

	channel.unsubscribe <- client

	client.notifyChannelLeave(channel, nil)
}

func (client *Client) joinChannel(message Message) {
	channelName := message.Name
	sender := message.Sender

	channel := client.wsServer.findChannelByName(channelName)

	if channel == nil {
		channel = client.wsServer.createChannel(channelName, sender == nil)
	}

	// Don't allow to join private channels through public channel message
	if sender != nil && channel.Private {
		log.Printf("Tried to join private channel through public channel message %s", channel.Name)
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
		Action:    ChannelJoinedAction,
		Event:     ChannelJoinedAction,
		Name:      channel.Name,
		Target:    channel,
		Sender:    sender,
		Timestamp: time.Now().Unix(),
	}

	client.send <- message.encode()
}

func (client *Client) notifyChannelLeave(channel *Channel, sender *Client) {
	message := Message{
		Action:    LeaveChannelAction,
		Event:     LeaveChannelAction,
		Name:      channel.Name,
		Target:    channel,
		Sender:    sender,
		Timestamp: time.Now().Unix(),
	}

	client.send <- message.encode()
}

func (client *Client) GetId() string {
	return client.ID.String()
}
