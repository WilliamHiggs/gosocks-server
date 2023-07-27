package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

type ChannelMessage struct {
	ChannelId string `json:"channel_id"`
	Data      string `json:"data"`
}

var (
	rdb *redis.Client
)

var clients = make(map[*websocket.Conn]bool)
var broadcaster = make(chan ChannelMessage)
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func checkToken(w http.ResponseWriter, r *http.Request) {
	reqToken := r.Header.Get("Authorization")
	splitToken := strings.Split(reqToken, "Bearer ")

	if len(splitToken) != 2 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimSpace(splitToken[1])

	if token != os.Getenv("AUTH_TOKEN") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Fatal(err)
	}

	defer ws.Close()

	checkToken(w, r)

	clients[ws] = true

	// if it's zero, no messages were ever sent/saved
	// this is not necessary for a subscription based websocket connection
	// maybe implement this later via a query param
	/*
		if rdb.Exists("channel_messages").Val() != 0 {
			sendPreviousMessages(ws)
		}
	*/

	for {
		var msg ChannelMessage
		// Read in a new message as JSON and map it to a Message object
		err := ws.ReadJSON(&msg)

		if err != nil {
			delete(clients, ws)
			break
		}
		// send new message to the channel
		broadcaster <- msg
	}
}

/*
func sendPreviousMessages(ws *websocket.Conn) {
	channelMessages, err := rdb.LRange("channel_messages", 0, -1).Result()

	if err != nil {
		panic(err)
	}

	// send previous messages
	for _, channelMessage := range channelMessages {
		var msg ChannelMessage
		json.Unmarshal([]byte(channelMessage), &msg)
		messageClient(ws, msg)
	}
}
*/

// If a message is sent while a client is closing, ignore the error
func unsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}

func handleMessages() {
	for {
		// grab any next message from channel
		msg := <-broadcaster

		storeInRedis(msg)
		messageClients(msg)
	}
}

func storeInRedis(msg ChannelMessage) {
	json, err := json.Marshal(msg)

	if err != nil {
		panic(err)
	}

	if err := rdb.RPush("channel_messages", json).Err(); err != nil {
		panic(err)
	}
}

func messageClients(msg ChannelMessage) {
	// send to every client currently connected
	for client := range clients {
		messageClient(client, msg)
	}
}

func messageClient(client *websocket.Conn, msg ChannelMessage) {
	err := client.WriteJSON(msg)
	if err != nil && unsafeError(err) {
		log.Printf("error: %v", err)
		client.Close()
		delete(clients, client)
	}
}

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")

	redisURL := os.Getenv("REDIS_URL")
	opt, err := redis.ParseURL(redisURL)

	if err != nil {
		panic(err)
	}

	rdb = redis.NewClient(opt)

	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	log.Print("Server starting at localhost:" + port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
