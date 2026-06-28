// Package parity hosts the differential test harness: it boots the legacy
// TypeScript backend and the Go backend against the same Postgres database and
// compares their HTTP responses (byte-parity for reads) and resulting rows
// (row-parity for writes).
package parity

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// RepoRoot returns the WorldSignal repository root, resolved from this file.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	// .../backend-go/internal/parity/harness.go → up 4 to repo root.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

// Server is a running backend under test.
type Server struct {
	BaseURL string
	cmd     *exec.Cmd
	logs    *bytes.Buffer
}

// Stop terminates the server process.
func (s *Server) Stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
}

// Logs returns captured stdout/stderr (for debugging failures).
func (s *Server) Logs() string {
	if s.logs == nil {
		return ""
	}
	return s.logs.String()
}

// StartTS launches the TypeScript backend (api role) on the given port against
// dbURL, with the LLM disabled. Returns once /health responds.
func StartTS(port int, dbURL string) (*Server, error) {
	backend := filepath.Join(RepoRoot(), "backend")
	cmd := exec.Command("node", "--import", "tsx", "src/server.ts")
	cmd.Dir = backend
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dbURL,
		"OPENAI_API_KEY=",
		"ROLE=api",
		fmt.Sprintf("PORT=%d", port),
		"HOST=127.0.0.1",
		"WEBHOOK_SIGNING_SECRET=parity-secret",
	)
	return start(cmd, port)
}

// StartGo launches the compiled Go backend (api role) on the given port.
func StartGo(binary string, port int, dbURL string) (*Server, error) {
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		"DATABASE_URL="+dbURL,
		"OPENAI_API_KEY=",
		"ROLE=api",
		fmt.Sprintf("PORT=%d", port),
		"HOST=127.0.0.1",
		"WEBHOOK_SIGNING_SECRET=parity-secret",
	)
	return start(cmd, port)
}

func start(cmd *exec.Cmd, port int) (*Server, error) {
	logs := &bytes.Buffer{}
	cmd.Stdout = logs
	cmd.Stderr = logs
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	s := &Server{BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port), cmd: cmd, logs: logs}
	if err := waitHealthy(s.BaseURL, 30*time.Second); err != nil {
		s.Stop()
		return nil, fmt.Errorf("%w\n--- server logs ---\n%s", err, logs.String())
	}
	return s, nil
}

func waitHealthy(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("server at %s not healthy within %s", base, timeout)
}

// Response is a captured HTTP response.
type Response struct {
	Status int
	Body   []byte
	Header http.Header
}

// Get issues a GET request.
func (s *Server) Get(path string) (*Response, error) { return s.do(http.MethodGet, path, "", nil) }

// GetGraphQL issues a GraphQL query over GET (query + optional JSON variables).
// GET is used for read parity because the legacy server's POST body handling
// hangs (Fastify drains the body before graphql-yoga reads it); the response
// body is identical to what POST would return.
func (s *Server) GetGraphQL(query string, variablesJSON string) (*Response, error) {
	u := url.Values{}
	u.Set("query", query)
	if variablesJSON != "" {
		u.Set("variables", variablesJSON)
	}
	return s.do(http.MethodGet, "/graphql?"+u.Encode(), "", nil)
}

// PostJSON issues a POST with a JSON body.
func (s *Server) PostJSON(path string, body []byte) (*Response, error) {
	return s.do(http.MethodPost, path, "application/json", body)
}

// Post issues a bodyless POST (no Content-Type), e.g. for action endpoints.
func (s *Server) Post(path string) (*Response, error) {
	return s.do(http.MethodPost, path, "", nil)
}

// PatchJSON issues a PATCH with a JSON body.
func (s *Server) PatchJSON(path string, body []byte) (*Response, error) {
	return s.do(http.MethodPatch, path, "application/json", body)
}

// Options issues an OPTIONS preflight request.
func (s *Server) Options(path string) (*Response, error) {
	return s.do(http.MethodOptions, path, "", nil)
}

func (s *Server) do(method, path, contentType string, body []byte) (*Response, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, s.BaseURL+path, r)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Body: b, Header: resp.Header}, nil
}
