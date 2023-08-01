package main

type WsServer struct {
	clients     map[*Client]bool
	subscribe   chan *Client
	unsubscribe chan *Client
	broadcast   chan []byte
	channels    map[*Channel]bool
}

// newWebsocketServer creates a new WsServer type
func newWebsocketServer() *WsServer {
	return &WsServer{
		clients:     make(map[*Client]bool),
		subscribe:   make(chan *Client),
		unsubscribe: make(chan *Client),
		broadcast:   make(chan []byte),
		channels:    make(map[*Channel]bool),
	}
}

// Run our websocket server, accepting various requests
func (server *WsServer) Run() {
	for {
		select {

		case client := <-server.subscribe:
			server.subscribeClient(client)

		case client := <-server.unsubscribe:
			server.unsubscribeClient(client)

		case message := <-server.broadcast:
			server.broadcastToClients(message)
		}

	}
}

func (server *WsServer) subscribeClient(client *Client) {
	server.notifyClientJoined(client)
	server.listOnlineClients(client)
	server.clients[client] = true
}

func (server *WsServer) unsubscribeClient(client *Client) {
	if _, ok := server.clients[client]; ok {
		delete(server.clients, client)
		server.notifyClientLeft(client)
	}
}

func (server *WsServer) notifyClientJoined(client *Client) {
	message := &Message{
		Action: MemberAddedAction,
		Event:  MemberAddedAction,
		Sender: client,
	}

	server.broadcastToClients(message.encode())
}

func (server *WsServer) notifyClientLeft(client *Client) {
	message := &Message{
		Action: MemberRemovedAction,
		Event:  MemberRemovedAction,
		Sender: client,
	}

	server.broadcastToClients(message.encode())
}

func (server *WsServer) listOnlineClients(client *Client) {
	for existingClient := range server.clients {
		message := &Message{
			Action: MemberAddedAction,
			Event:  MemberAddedAction,
			Sender: existingClient,
		}
		client.send <- message.encode()
	}
}

func (server *WsServer) broadcastToClients(message []byte) {
	for client := range server.clients {
		client.send <- message
	}
}

func (server *WsServer) findChannelByName(name string) *Channel {
	var foundChannel *Channel
	for channel := range server.channels {
		if channel.GetName() == name {
			foundChannel = channel
			break
		}
	}

	return foundChannel
}

func (server *WsServer) findChannelByID(ID string) *Channel {
	var foundChannel *Channel
	for channel := range server.channels {
		if channel.GetId() == ID {
			foundChannel = channel
			break
		}
	}

	return foundChannel
}

func (server *WsServer) createChannel(name string, private bool) *Channel {
	channel := NewChannel(name, private)
	go channel.RunChannel()
	server.channels[channel] = true

	return channel
}

func (server *WsServer) findClientByID(ID string) *Client {
	var foundClient *Client
	for client := range server.clients {
		if client.ID.String() == ID {
			foundClient = client
			break
		}
	}

	return foundClient
}
