package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/jsonx"
)

type createSourceBody struct {
	Name           *string  `json:"name"`
	URL            *string  `json:"url"`
	Type           *string  `json:"type"`
	Country        *string  `json:"country"`
	Priority       *int     `json:"priority"`
	CrawlFrequency *int     `json:"crawlFrequency"`
	Credibility    *float64 `json:"credibility"`
}

func (s *Server) createSourceREST(w http.ResponseWriter, r *http.Request) {
	var b createSourceBody
	if err := readJSON(r, &b); err != nil || b.Name == nil || *b.Name == "" || b.URL == nil || *b.URL == "" {
		writeJSON(w, http.StatusBadRequest, struct {
			Error string `json:"error"`
		}{"name and url required"})
		return
	}
	in := db.CreateSourceInput{
		Name: *b.Name, URL: *b.URL,
		Type: "RSS", Priority: 5, CrawlFrequency: 900, Credibility: 0.5,
		Country: b.Country,
	}
	if b.Type != nil {
		in.Type = *b.Type
	}
	if b.Priority != nil {
		in.Priority = *b.Priority
	}
	if b.CrawlFrequency != nil {
		in.CrawlFrequency = *b.CrawlFrequency
	}
	if b.Credibility != nil {
		in.Credibility = *b.Credibility
	}

	src, err := s.DB.CreateSource(r.Context(), in)
	if err != nil {
		if errors.Is(err, db.ErrDuplicateURL) {
			writeJSON(w, http.StatusConflict, struct {
				Error string `json:"error"`
			}{"source url already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.Enqueue.EnqueueFetchSource(src.ID) // fetch immediately
	writeJSON(w, http.StatusCreated, src)
}

type patchSourceBody struct {
	Enabled        *bool `json:"enabled"`
	Priority       *int  `json:"priority"`
	CrawlFrequency *int  `json:"crawlFrequency"`
}

func (s *Server) patchSourceREST(w http.ResponseWriter, r *http.Request) {
	var b patchSourceBody
	_ = readJSON(r, &b)
	src, err := s.DB.UpdateSource(r.Context(), r.PathValue("id"), db.SourcePatch{
		Enabled: b.Enabled, Priority: b.Priority, CrawlFrequency: b.CrawlFrequency,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) fetchSourceREST(w http.ResponseWriter, r *http.Request) {
	_ = s.Enqueue.EnqueueFetchSource(r.PathValue("id"))
	writeJSON(w, http.StatusOK, struct {
		Queued bool `json:"queued"`
	}{true})
}

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	return jsonx.Unmarshal(body, v)
}
