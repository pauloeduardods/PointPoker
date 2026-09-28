package handler

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pauloedsg/pointpoker/internal/service"
)

// errorResponse is the body of every error response.
type errorResponse struct {
	Error string `json:"error"`
}

func abortError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: msg})
}

// writeError maps a service error to its HTTP status. Unexpected errors are
// logged and reported as a generic 500.
func writeError(c *gin.Context, err error) {
	var status int
	switch {
	case errors.Is(err, service.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
	default:
		log.Printf("%s %s: internal error: %v", c.Request.Method, c.FullPath(), err)
		abortError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	abortError(c, status, err.Error())
}

// bindJSON decodes the request body into dst, answering 400 on failure.
func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		abortError(c, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func roomCode(c *gin.Context) string {
	return service.NormalizeCode(c.Param("code"))
}

func sessionToken(c *gin.Context) string {
	return c.GetHeader(SessionTokenHeader)
}
