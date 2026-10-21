package api

import "net/http"

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/web/", http.StatusSeeOther)
}
