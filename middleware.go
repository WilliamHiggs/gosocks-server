package main

import (
	"log"
	"net/http"
	"os"
)

func middleware(f http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, tok := r.URL.Query()["bearer"]
		authToken := os.Getenv("AUTH_TOKEN")

		if len(authToken) == 0 {
			log.Println("An authentication token is required to use this application.")
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		if !tok || len(token[0]) < 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if token[0] != authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		f(w, r)
	})
}
