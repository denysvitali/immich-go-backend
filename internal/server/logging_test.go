package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLoggedQueryKeysNeverIncludesValues(t *testing.T) {
	raw := "token=bearer-secret&password=hunter2&assetId=asset-123&assetId=asset-456&code=oauth-code"

	got := loggedQueryKeys(raw)

	assert.Equal(t, "assetId,code,password,token", got)
	assert.NotContains(t, got, "bearer-secret")
	assert.NotContains(t, got, "hunter2")
	assert.NotContains(t, got, "oauth-code")
	assert.NotContains(t, got, "asset-123")
}

func TestLoggedQueryKeysHandlesMalformedInputWithoutEchoingIt(t *testing.T) {
	assert.Equal(t, "<malformed>", loggedQueryKeys("token=%zz-secret"))
}

func TestRequestLoggingHandlerRespectsConfiguration(t *testing.T) {
	logger := logrus.StandardLogger()
	previousOutput := logger.Out
	previousFormatter := logger.Formatter
	previousLevel := logger.Level
	t.Cleanup(func() {
		logger.SetOutput(previousOutput)
		logger.SetFormatter(previousFormatter)
		logger.SetLevel(previousLevel)
	})

	var output bytes.Buffer
	logger.SetOutput(&output)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	logger.SetLevel(logrus.InfoLevel)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	requestLoggingHandler(false, next).ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/quiet", nil),
	)
	assert.Empty(t, output.String())

	requestLoggingHandler(true, next).ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/logged", nil),
	)
	assert.Contains(t, output.String(), "Handled request")
	assert.Contains(t, output.String(), "path=/logged")
}
