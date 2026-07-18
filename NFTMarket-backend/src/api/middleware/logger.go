package middleware

import (
	"bytes"
	"io"
	"io/ioutil"
	"time"

	"github.com/ProjectsTask/EasySwapBase/logger/xzap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type BodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w BodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
func (w BodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// RLog 请求响应日志打印处理
func RLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		// some evil middlewares modify this values
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		var buf bytes.Buffer
		tee := io.TeeReader(c.Request.Body, &buf)
		requestBody, _ := ioutil.ReadAll(tee)
		c.Request.Body = ioutil.NopCloser(&buf)
		bodyLogWriter := &BodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bodyLogWriter

		start := time.Now()

		c.Next()

		responseBody := bodyLogWriter.body.Bytes()
		logger := xzap.WithContext(c.Request.Context())
		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			for _, e := range c.Errors.Errors() {
				logger.Error(e)
			}
		} else {
			latency := float64(time.Now().Sub(start).Nanoseconds() / 1000000.0)
			fields := []zapcore.Field{
				zap.Int("status", c.Writer.Status()),
				zap.String("method", c.Request.Method),
				zap.String("function", c.HandlerName()),
				zap.String("path", path),
				zap.String("query", query),
				zap.String("ip", c.ClientIP()),
				zap.String("user-agent", c.Request.UserAgent()),
				zap.String("token", c.Request.Header.Get("session_id")),
				zap.String("content-type", c.Request.Header.Get("Content-Type")),
				zap.Float64("latency", latency),
				zap.String("request", string(requestBody)),
				zap.String("response", string(responseBody)),
			}
			logger.Info("Go-End", fields...)
		}
	}
}
