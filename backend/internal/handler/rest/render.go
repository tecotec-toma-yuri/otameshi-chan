package rest

import (
	"net/http"

	"github.com/go-chi/render"
)

func renderJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	render.Status(r, status)
	render.JSON(w, r, v)
}

func renderError(w http.ResponseWriter, r *http.Request, status int, err error) {
	render.Status(r, status)
	render.JSON(w, r, map[string]string{"error": err.Error()})
}
