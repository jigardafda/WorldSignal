package httpapi

import "net/http"

func (s *Server) registerSourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/sources", s.listSources)
}

// listSources mirrors GET /v1/sources → { data: rows } where rows are raw Source
// objects ordered by priority asc, name asc.
func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.ListSources(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rows})
}
