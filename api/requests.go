package api

type UpdateMessageParams struct {
	IsRead bool `json:"is_read"`
}

type UpdateMessage struct {
	Message UpdateMessageParams `json:"message"`
}
