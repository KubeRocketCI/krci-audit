package api

import (
	"log/slog"
	"net/http"
	"strconv"
)

// errorCode renders an HTTP status as the API's numeric error code string.
func errorCode(status int) string { return strconv.Itoa(status) }

// serverErrorResponse logs err server-side and returns a generic code/message pair for a 500
// response, so internal error detail (SQL text, driver messages) never reaches API clients.
func serverErrorResponse(err error) (code, message string) {
	slog.Error("api request failed", "error", err)
	return errorCode(http.StatusInternalServerError), http.StatusText(http.StatusInternalServerError)
}
