package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Error struct {
	StatusCode int    `json:"-"`
	Message    string `json:"msg"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

var (
	AuthenticationError = Error{StatusCode: http.StatusUnauthorized, Message: "AuthenticationError Error"}
	AuthorizationError  = Error{StatusCode: http.StatusForbidden, Message: "Authorization Error"}
	NotFoundError       = Error{StatusCode: http.StatusNotFound, Message: "Not Found Error"}
	BadRequestError     = Error{StatusCode: http.StatusBadRequest, Message: "Bad request"}
	ServerError         = Error{StatusCode: http.StatusInternalServerError, Message: "Internal Server Error"}
)

func Abort500(c *gin.Context, err error) {
	res := &ErrorResponse{Error: ServerError}
	if err != nil {
		res.Error.Message = err.Error()
	}
	c.JSON(res.Error.StatusCode, res)
	c.Abort()
}

func AbortBadRequestError(c *gin.Context, err error) {
	res := &ErrorResponse{Error: BadRequestError}
	if err != nil {
		res.Error.Message = err.Error()
	}
	c.JSON(res.Error.StatusCode, res)
	c.Abort()
}

func Abort401(c *gin.Context, err error) {
	res := &ErrorResponse{Error: AuthenticationError}
	if err != nil {
		res.Error.Message = err.Error()
	}
	c.JSON(res.Error.StatusCode, res)
	c.Abort()
}

func Abort403(c *gin.Context, err error) {
	res := &ErrorResponse{Error: AuthorizationError}
	if err != nil {
		res.Error.Message = err.Error()
	}
	c.JSON(res.Error.StatusCode, res)
	c.Abort()
}

func Abort404(c *gin.Context, err error) {
	res := &ErrorResponse{Error: NotFoundError}
	if err != nil {
		res.Error.Message = err.Error()
	}
	c.JSON(res.Error.StatusCode, res)
	c.Abort()
}
