package main

import (
	"github.com/google/uuid"
)

const connectionMessage = "%s joined the channel"

type Channel struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	clients     map[*Client]bool
	subscribe   chan *Client
	unsubscribe chan *Client
	broadcast   chan *Message
	Private     bool `json:"private"`
}

// NewChannel creates a new Channel
func NewChannel(name string, private bool) *Channel {
	return &Channel{
		ID:          uuid.New(),
		Name:        name,
		clients:     make(map[*Client]bool),
		subscribe:   make(chan *Client),
		unsubscribe: make(chan *Client),
		broadcast:   make(chan *Message),
		Private:     private,
	}
}

// RunChannel runs our channel, accepting various requests
func (channel *Channel) RunChannel() {
	for {
		select {

		case client := <-channel.subscribe:
			channel.subscribeClientInChannel(client)

		case client := <-channel.unsubscribe:
			channel.unsubscribeClientInChannel(client)

		case message := <-channel.broadcast:
			channel.broadcastToClientsInChannel(message.encode())
		}
	}
}

func (channel *Channel) subscribeClientInChannel(client *Client) {
	channel.notifyClientJoined(client)
	channel.clients[client] = true
}

func (channel *Channel) unsubscribeClientInChannel(client *Client) {
	if _, ok := channel.clients[client]; ok {
		delete(channel.clients, client)
	}
}

func (channel *Channel) broadcastToClientsInChannel(message []byte) {
	for client := range channel.clients {
		client.send <- message
	}
}

func (channel *Channel) notifyClientJoined(client *Client) {
	clientId := ""

	if !channel.Private {
		clientId = ":" + client.GetId()
	}

	message := &Message{
		Action: SendMessageAction,
		Name:   channel.Name,
		Event:  MemberAddedAction + clientId,
		Target: channel,
	}

	channel.broadcastToClientsInChannel(message.encode())
}

func (channel *Channel) GetId() string {
	return channel.ID.String()
}

func (channel *Channel) GetName() string {
	return channel.Name
}
