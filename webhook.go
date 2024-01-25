package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

func webhook(clientId string, event string) {
	url := os.Getenv("WEBHOOK_URL")

	if len(url) == 0 {
		return
	}

	data := []byte(`{"client_id":"` + clientId + `","event":"` + event + `"}`)

	// create new http request
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))

	if err != nil {
		log.Printf("Error on sending client webhook %s", err)
		return
	}

	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	// send the request
	client := &http.Client{}
	response, err := client.Do(request)

	if err != nil {
		log.Printf("Error on sending client webhook %s", err)
		return
	}

	responseBody, err := io.ReadAll(response.Body)

	if err != nil {
		log.Printf("Error on sending client webhook %s", err)
		return
	}

	formattedData := formatJSON(responseBody)
	log.Printf("Status: %s", response.Status)
	log.Printf("Response body: %s", formattedData)

	// clean up memory after execution
	defer response.Body.Close()
}

func formatJSON(data []byte) string {
	var out bytes.Buffer
	err := json.Indent(&out, data, "", " ")

	if err != nil {
		log.Printf("Webhook Format JSON Error: %s", err)
	}

	d := out.Bytes()
	return string(d)
}
