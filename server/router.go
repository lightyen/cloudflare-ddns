package server

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lightyen/cloudflare-ddns/settings"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

func recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			e := recover()
			if e == nil {
				return
			}

			if err, ok := e.(error); ok {
				if errors.Is(err, http.ErrAbortHandler) {
					panic(e)
				}

				log.Error(err)
				return
			}

			log.Error(InternalServerError(e))
		}()

		c.Next()
	}
}

func (s *Server) buildRouter() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(recovery())
	e.NoRoute(s.fileServe())

	api := e.Group("/vapi")
	{
		api.GET("/version", func(c *gin.Context) {
			c.String(http.StatusOK, settings.Version)
		})

		api.GET("/logs", s.GetLogs)
		api.DELETE("/logs", s.DeleteLogs)

		api.POST("/records/apply", func(c *gin.Context) {
			s.apply <- struct{}{}
			c.JSON(200, struct{}{})
		})
	}

	return e
}
