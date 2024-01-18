package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.FileServer(http.Dir("./public"))
}

func main() {
	err := godotenv.Load()

	log.SetOutput(os.Stdout)

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")

	server := newWebsocketServer()

	go server.Run()

	http.HandleFunc("/", serveHome)

	http.HandleFunc("/ws", middleware(func(w http.ResponseWriter, r *http.Request) {
		serveWs(server, w, r)
	}))

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
