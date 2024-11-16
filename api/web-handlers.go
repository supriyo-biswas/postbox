package api

import (
	"log"
	"net/http"

	ent "github.com/supriyo-biswas/postbox/entities"
)

func (s *Server) webApiGetInfo(w http.ResponseWriter, r *http.Request) {
	_, p, _ := r.BasicAuth()
	inbox := r.Context().Value(inboxContextKey).(*ent.Inbox)
	res, err := s.buildInboxResponse(inbox)
	if err != nil {
		log.Printf("failed to get inbox %d: %s", inbox.Id, err)
		sendError(w, http.StatusInternalServerError, internalServerErrorMsg)
		return
	}

	sendResponse(w, http.StatusOK, WebInbox{Inbox: res, Token: p})
}

func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/web/", http.StatusSeeOther)
}
