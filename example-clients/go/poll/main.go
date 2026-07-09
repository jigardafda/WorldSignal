// Consume a WorldSignal subscription by polling — stdlib only.
//
// Run: go run ./poll   (from example-clients/go)
// Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX,
//
//	WS_INTERVAL (seconds, default 3).
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

type pollEvent struct {
	Payload struct {
		Data struct {
			Severity string `json:"severity"`
			Country  string `json:"country"`
			Title    string `json:"title"`
		} `json:"data"`
	} `json:"payload"`
}
type pollResp struct {
	Events []pollEvent `json:"events"`
	Cursor int64       `json:"cursor"`
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	base := env("WS_API_BASE", "http://localhost:4800")
	key := os.Getenv("WS_API_KEY")
	sub := env("WS_SUBSCRIPTION", "demo-stream")
	max, _ := strconv.Atoi(env("WS_MAX", "0"))
	interval, _ := strconv.Atoi(env("WS_INTERVAL", "3"))
	if key == "" {
		fmt.Fprintln(os.Stderr, "WS_API_KEY is required")
		os.Exit(2)
	}
	cursor, _ := strconv.ParseInt(env("WS_SINCE", "0"), 10, 64)
	seen := 0
	fmt.Fprintf(os.Stderr, "[poll] %s/v1/stream/poll subscription=%s\n", base, sub)
	for {
		url := fmt.Sprintf("%s/v1/stream/poll?subscription=%s&since=%d", base, sub, cursor)
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var body pollResp
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			resp.Body.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		resp.Body.Close()
		for _, ev := range body.Events {
			d := ev.Payload.Data
			fmt.Printf("[poll] %-8s %s  %s\n", or(d.Severity, "?"), or(d.Country, "--"), d.Title)
			seen++
			if max != 0 && seen >= max {
				return
			}
		}
		cursor = body.Cursor
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
