package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

const requestIDHeaderKey = "Request_ID"

func ServerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		reqID := uuid.NewString()
		ctxWithID := context.WithValue(req.Context(), "request_id", reqID)
		reqWithID := req.WithContext(ctxWithID)
		w.Header().Set(requestIDHeaderKey, reqID)
		next.ServeHTTP(w, reqWithID)
	})
}
