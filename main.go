package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

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

func checkToken(t string, w http.ResponseWriter, r *http.Request) error {
	reqToken := r.Header.Get("Authorization")
	splitToken := strings.Split(reqToken, "Bearer ")

	if len(splitToken) != 2 {
		return errors.New("No token found")
	}

	token := strings.TrimSpace(splitToken[1])

	if t != token {
		return errors.New("Unauthorized")
	}

	return nil
}

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	port := os.Getenv("PORT")
	//addr := os.Getenv("PUBLIC_URL")
	token := os.Getenv("AUTH_TOKEN")

	server := newWebsocketServer()

	go server.Run()

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if err := checkToken(token, w, r); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		} else {
			serveWs(server, w, r)
		}
	})

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
