package api

import (
	"net/http"

	ent "github.com/supriyo-biswas/postbox/entities"
)

func (s *Server) webApiGetInfo(w http.ResponseWriter, r *http.Request) {
	inbox := r.Context().Value(inboxContextKey).(*ent.Inbox)
	s.sendInboxResponse(w, inbox)
}

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/web/", http.StatusSeeOther)
}
