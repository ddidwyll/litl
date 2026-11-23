package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type ProcessRequest struct {
	Action string `json:"action"`
	Text   string `json:"text,omitempty"`
}

type ProcessResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func main() {
	action := flag.String("action", "", "Action")
	port := flag.Int("port", 8765, "Server port")
	token := flag.String("token", "", "Auth token")
	text := flag.String("text", "", "Text to process (optional, default from clipboard)")
	flag.Parse()

	if *action == "" {
		fmt.Println("Usage: litl --action <name> [--text <text>]")
		os.Exit(1)
	}

	// TOKEN ENV
	authToken := *token
	if authToken == "" {
		authToken = os.Getenv("LITL_AUTH_TOKEN")
	}

	reqBody := ProcessRequest{Action: *action, Text: *text}
	jsonData, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 60 * time.Second}
	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("http://127.0.0.1:%d/process", *port),
		bytes.NewBuffer(jsonData),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to daemon: %v\n", err)
		fmt.Fprintln(os.Stderr, "Is the daemon running?")
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result ProcessResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid response: %s\n", body)
		os.Exit(1)
	}

	if result.Error != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", result.Error)
		os.Exit(1)
	}

	// REMOVE_ME
	fmt.Print(result.Result)
}
