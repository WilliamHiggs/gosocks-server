package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	request, error := http.NewRequest("POST", url, bytes.NewBuffer(data))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")

	// send the request
	client := &http.Client{}
	response, error := client.Do(request)

	if error != nil {
		fmt.Println(error)
	}

	responseBody, error := io.ReadAll(response.Body)

	if error != nil {
		fmt.Println(error)
	}

	formattedData := formatJSON(responseBody)
	fmt.Println("Status: ", response.Status)
	fmt.Println("Response body: ", formattedData)

	// clean up memory after execution
	defer response.Body.Close()
}

func formatJSON(data []byte) string {
	var out bytes.Buffer
	err := json.Indent(&out, data, "", " ")

	if err != nil {
		fmt.Println(err)
	}

	d := out.Bytes()
	return string(d)
}
