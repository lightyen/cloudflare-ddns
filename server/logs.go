package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/lightyen/cloudflare-ddns/settings"
	"github.com/lightyen/cloudflare-ddns/zok/log"
)

func (s *Server) GetLogs(c *gin.Context) {
	filename := filepath.Join(settings.Value().DataDirectory, log.Filename())
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()

	items := []*log.LogEntry{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		var v *log.LogEntry
		if err := json.Unmarshal([]byte(line), &v); err == nil {
			items = append(items, v)
		}
	}

	c.JSON(http.StatusOK, items)
}

func (s *Server) DeleteLogs(c *gin.Context) {
	if err := log.Rotate(); err != nil {
		Abort500(c, err)
		return
	}
	c.Status(http.StatusOK)
}
