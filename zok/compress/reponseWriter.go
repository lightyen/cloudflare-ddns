package compress

import (
	"compress/gzip"
	"io"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"
	"github.com/lightyen/cloudflare-ddns/zok/header"
)

var (
	gzPool = sync.Pool{
		New: func() interface{} {
			gz, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
			if err != nil {
				panic(err)
			}
			return gz
		},
	}
)

type zWriter struct {
	gin.ResponseWriter
	writer io.Writer
}

func (g *zWriter) WriteString(s string) (int, error) {
	g.Header().Del("Content-Length")
	return g.writer.Write([]byte(s))
}

func (g *zWriter) Write(data []byte) (int, error) {
	g.Header().Del("Content-Length")
	return g.writer.Write(data)
}

func (g *zWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length")
	g.ResponseWriter.WriteHeader(code)
}

type zCloser struct {
	close func() error
}

func (z *zCloser) Close() error {
	if z.close == nil {
		return nil
	}
	return z.close()
}

func CompressResponseWriter(c *gin.Context) io.Closer {
	h := header.ParseAcceptEncoding(c.Request.Header.Get("Accept-Encoding"))

	switch {
	case h.Contains("zstd"):
		c.Header("Content-Encoding", "zstd")
		c.Header("Vary", "Accept-Encoding")

		zw, _ := zstd.NewWriter(c.Writer)
		c.Writer = &zWriter{c.Writer, zw}
		return zw
	case h.Contains("gzip"):
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")

		gz := gzPool.Get().(*gzip.Writer)

		gz.Reset(c.Writer)

		c.Writer = &zWriter{c.Writer, gz}

		return &zCloser{close: func() error {
			err := gz.Close()
			gz.Reset(io.Discard)
			gzPool.Put(gz)
			return err
		}}
	}

	return &zCloser{}
}
