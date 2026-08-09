package server

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func requestLoggingHandler(enabled bool, next http.Handler) http.Handler {
	if !enabled {
		return next
	}
	return httpLoggingHandler(next)
}

func httpLoggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseStatusRecorder{ResponseWriter: w}
		start := time.Now()

		next.ServeHTTP(rec, r)

		statusCode := rec.status
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		entry := logrus.WithFields(logrus.Fields{
			"bytes":       rec.bytes,
			"durationMs":  time.Since(start).Milliseconds(),
			"method":      r.Method,
			"path":        r.URL.Path,
			"queryParams": loggedQueryKeys(r.URL.RawQuery),
			"status":      statusCode,
			"userAgent":   r.UserAgent(),
			"requestHost": r.Host,
		})
		switch {
		case statusCode >= http.StatusInternalServerError:
			entry.Error("Handled request")
		case statusCode >= http.StatusBadRequest:
			entry.Warn("Handled request")
		default:
			entry.Info("Handled request")
		}
	})
}

// loggedQueryKeys keeps request logs useful without recording query values.
// Several Immich routes accept bearer-equivalent shared-link credentials and
// OAuth codes in the query string, so even an allow-list of secret field names
// is too easy to get wrong as the API evolves.
func loggedQueryKeys(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "<malformed>"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

type responseStatusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *responseStatusRecorder) WriteHeader(statusCode int) {
	if r.status != 0 {
		return
	}
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseStatusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += int64(n)
	return n, err
}

func (r *responseStatusRecorder) Flush() {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *responseStatusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (r *responseStatusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (r *responseStatusRecorder) ReadFrom(src io.Reader) (int64, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if readerFrom, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		n, err := readerFrom.ReadFrom(src)
		r.bytes += n
		return n, err
	}
	return io.Copy(responseRecorderWriter{r}, src)
}

func (r *responseStatusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

type responseRecorderWriter struct {
	rec *responseStatusRecorder
}

func (w responseRecorderWriter) Write(data []byte) (int, error) {
	return w.rec.Write(data)
}
