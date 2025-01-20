package gzip

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

var AllowedContentTypes = []string{"text/html", "application/json"}

const minLength = 150

type bufferedResponseWriter struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (w *bufferedResponseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bufferedResponseWriter) Write(b []byte) (int, error) {
	if w.body == nil {
		w.body = bytes.NewBuffer(b)
	} else {
		w.body.Write(b)
	}
	return len(b), nil
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

func GzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		acceptEncoding := c.Request.Header.Get("Accept-Encoding")

		originalWriter := c.Writer
		bufferedWriter := &bufferedResponseWriter{
			ResponseWriter: originalWriter,
		}
		c.Writer = bufferedWriter

		c.Next()

		contentType := bufferedWriter.Header().Get("Content-Type")
		if strings.Contains(acceptEncoding, "gzip") &&
			bufferedWriter.status < 300 &&
			slices.Contains(AllowedContentTypes, contentType) &&
			bufferedWriter.body.Len() > minLength {
			c.Header("Content-Encoding", "gzip")
			c.Header("Vary", "Accept-Encoding")
			c.Header("Content-Length", "")

			gw, err := gzip.NewWriterLevel(originalWriter, gzip.BestSpeed)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer gw.Close()

			_, err = gw.Write(bufferedWriter.body.Bytes())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		} else if bufferedWriter.body != nil {
			_, err := originalWriter.Write(bufferedWriter.body.Bytes())
			if err != nil {
				c.IndentedJSON(http.StatusInternalServerError, err)
				return
			}
		}
	}
}
