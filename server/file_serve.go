package server

import (
	"crypto/md5"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lightyen/cloudflare-ddns/config"
	"github.com/lightyen/cloudflare-ddns/zok/compress"
	"github.com/lightyen/cloudflare-ddns/zok/header"
)

func etag(filename string) (string, error) {
	h := md5.New()
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return strconv.Quote(base64.StdEncoding.EncodeToString(h.Sum(nil))), nil
}

func fileExists(root, urlpath string) bool {
	if filename := strings.TrimPrefix(urlpath, "/"); len(filename) < len(urlpath) {
		name := filepath.Join(root, filename)
		stats, err := os.Stat(name)
		if err != nil {
			return false
		}
		return stats.Mode().IsRegular()
	}
	return false
}

func (s *Server) returnIndex(useAny bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		index := filepath.Join(config.Config().DataDirectory, config.Config().WebRoot, "index.html")
		_, err := os.Stat(index)
		if err != nil {
			return
		}

		a := header.ParseAccept(c.Request.Header.Get("Accept"))

		if a.Contains("text/html") || (useAny && a.Contains("*/*")) {
			eTag, _ := etag(index)
			im := c.Request.Header.Get("If-Match")
			if im != "" && im == eTag {
				c.Status(http.StatusNotModified)
				return
			}

			if eTag != "" {
				c.Header("Cache-Control", "max-age=0, private, must-revalidate")
				c.Header("Etag", eTag)
			}

			defer compress.CompressResponseWriter(c).Close()

			c.File(index)
		}
	}
}

func (s *Server) fileServe() gin.HandlerFunc {
	root := filepath.Join(config.Config().DataDirectory, config.Config().WebRoot)
	serve := http.StripPrefix("/", http.FileServer(gin.Dir(root, false)))
	index := s.returnIndex(true)

	return func(c *gin.Context) {
		if fileExists(root, c.Request.URL.Path) {
			filename := filepath.Join(config.Config().DataDirectory, config.Config().WebRoot, c.Request.URL.Path)
			if eTag, _ := etag(filename); eTag != "" {
				c.Header("Cache-Control", "max-age=0")
				c.Header("Etag", eTag)
			}

			defer compress.CompressResponseWriter(c).Close()

			serve.ServeHTTP(c.Writer, c.Request)
			return
		}

		if c.Request.Method != http.MethodGet {
			// TODO: custom not found page
			return
		}

		index(c)
	}
}
