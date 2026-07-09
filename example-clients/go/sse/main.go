// Consume a WorldSignal subscription as Server-Sent Events — stdlib only.
//
// Run: go run ./sse   (from example-clients/go)
// Env: WS_API_BASE, WS_API_KEY, WS_SUBSCRIPTION, WS_SINCE, WS_MAX.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type envelope struct {
	Data struct {
		Severity string `json:"severity"`
		Country  string `json:"country"`
		Title    string `json:"title"`
	} `json:"data"`
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func main() {
	base := env("WS_API_BASE", "http://localhost:4800")
	key := os.Getenv("WS_API_KEY")
	sub := env("WS_SUBSCRIPTION", "demo-stream")
	since := env("WS_SINCE", "0")
	max, _ := strconv.Atoi(env("WS_MAX", "0"))
	if key == "" {
		fmt.Fprintln(os.Stderr, "WS_API_KEY is required")
		os.Exit(2)
	}
	url := fmt.Sprintf("%s/v1/stream/sse?subscription=%s&since=%s", base, sub, since)
	fmt.Fprintf(os.Stderr, "[sse] connecting to %s\n", url)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	seen := 0
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if !strings.HasPrefix(line, "data:") {
			continue // skip id:/event:/comment lines
		}
		var ev envelope
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[len("data:"):])), &ev); err != nil {
			continue
		}
		d := ev.Data
		fmt.Printf("[sse] %-8s %s  %s\n", or(d.Severity, "?"), or(d.Country, "--"), d.Title)
		seen++
		if max != 0 && seen >= max {
			fmt.Fprintf(os.Stderr, "[sse] received %d event(s), exiting\n", seen)
			return
		}
	}
}
