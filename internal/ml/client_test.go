package ml

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEmbeddingValueArray(t *testing.T) {
	emb, err := parseEmbeddingValue(json.RawMessage(`[0.1, 0.2, -0.5]`))
	require.NoError(t, err)
	require.Len(t, emb, 3)
	assert.InDelta(t, 0.1, emb[0], 1e-6)
	assert.InDelta(t, 0.2, emb[1], 1e-6)
	assert.InDelta(t, -0.5, emb[2], 1e-6)
}

func TestParseEmbeddingValueStringWrapped(t *testing.T) {
	// Immich ML serializes numpy arrays as a JSON string containing a JSON array.
	raw, err := json.Marshal("[1.5, 2.5, 3.5]")
	require.NoError(t, err)

	emb, err := parseEmbeddingValue(raw)
	require.NoError(t, err)
	require.Len(t, emb, 3)
	assert.InDelta(t, 1.5, emb[0], 1e-6)
	assert.InDelta(t, 2.5, emb[1], 1e-6)
	assert.InDelta(t, 3.5, emb[2], 1e-6)
}

func TestParseCLIPEmbedding(t *testing.T) {
	body := `{"clip":"[0.25,-0.75]","imageHeight":100,"imageWidth":200}`
	emb, err := parseCLIPEmbedding(json.RawMessage(body))
	require.NoError(t, err)
	require.Len(t, emb, 2)
	assert.InDelta(t, 0.25, emb[0], 1e-6)
	assert.InDelta(t, -0.75, emb[1], 1e-6)
}

func TestParseCLIPEmbeddingArrayForm(t *testing.T) {
	body := `{"clip":[0.25,-0.75],"imageHeight":100,"imageWidth":200}`
	emb, err := parseCLIPEmbedding(json.RawMessage(body))
	require.NoError(t, err)
	require.Len(t, emb, 2)
}

func TestParseCLIPEmbeddingMissing(t *testing.T) {
	_, err := parseCLIPEmbedding(json.RawMessage(`{"imageHeight":1}`))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBadResponse)
}

func TestParseFaceDetection(t *testing.T) {
	body := `{
		"facial-recognition": [
			{
				"boundingBox": {"x1": 10.2, "y1": 20.7, "x2": 100.1, "y2": 200.9},
				"embedding": "[0.1,0.2,0.3]",
				"score": 0.92
			},
			{
				"boundingBox": {"x1": 1, "y1": 2, "x2": 3, "y2": 4},
				"embedding": [0.4, 0.5],
				"score": 0.81
			}
		],
		"imageHeight": 1080,
		"imageWidth": 1920
	}`
	result, err := parseFaceDetection(json.RawMessage(body))
	require.NoError(t, err)
	assert.Equal(t, 1920, result.ImageWidth)
	assert.Equal(t, 1080, result.ImageHeight)
	require.Len(t, result.Faces, 2)

	assert.Equal(t, int32(10), result.Faces[0].BoundingBox.X1)
	assert.Equal(t, int32(20), result.Faces[0].BoundingBox.Y1)
	assert.Equal(t, int32(100), result.Faces[0].BoundingBox.X2)
	assert.Equal(t, int32(200), result.Faces[0].BoundingBox.Y2)
	require.Len(t, result.Faces[0].Embedding, 3)
	assert.InDelta(t, 0.92, result.Faces[0].Score, 1e-6)
	assert.NotEmpty(t, result.Faces[0].EmbeddingRaw)

	require.Len(t, result.Faces[1].Embedding, 2)
	assert.InDelta(t, 0.4, result.Faces[1].Embedding[0], 1e-6)
}

func TestParseFaceDetectionEmpty(t *testing.T) {
	body := `{"facial-recognition": [], "imageHeight": 10, "imageWidth": 20}`
	result, err := parseFaceDetection(json.RawMessage(body))
	require.NoError(t, err)
	assert.Empty(t, result.Faces)
	assert.Equal(t, 20, result.ImageWidth)
}

func TestFormatVector(t *testing.T) {
	assert.Equal(t, "[]", FormatVector(nil))
	assert.Equal(t, "[0.1,0.2,-0.5]", FormatVector([]float32{0.1, 0.2, -0.5}))
}

func TestClientDisabled(t *testing.T) {
	c := NewClient(Config{Enabled: false, URL: "http://localhost:3003"})
	assert.False(t, c.Enabled())

	_, err := c.EncodeText(context.Background(), "cat", "")
	assert.ErrorIs(t, err, ErrDisabled)

	_, err = c.EncodeImage(context.Background(), []byte("fake"), "")
	assert.ErrorIs(t, err, ErrDisabled)

	_, err = c.DetectFaces(context.Background(), []byte("fake"), "", 0)
	assert.ErrorIs(t, err, ErrDisabled)
}

func TestClientEmptyURLDisabled(t *testing.T) {
	c := NewClient(Config{Enabled: true, URL: ""})
	assert.False(t, c.Enabled())
}

func TestEncodeTextHTTP(t *testing.T) {
	var gotEntries string
	var gotText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/predict", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		gotEntries = r.FormValue("entries")
		gotText = r.FormValue("text")
		_, _ = w.Write([]byte(`{"clip":"[0.11,0.22,0.33]"}`))
	}))
	defer srv.Close()

	c := NewClient(Config{Enabled: true, URL: srv.URL, CLIPModel: "test-model"})
	emb, err := c.EncodeText(context.Background(), "a dog running", "")
	require.NoError(t, err)
	require.Len(t, emb, 3)
	assert.InDelta(t, 0.11, emb[0], 1e-6)
	assert.Equal(t, "a dog running", gotText)
	assert.Contains(t, gotEntries, `"clip"`)
	assert.Contains(t, gotEntries, `"textual"`)
	assert.Contains(t, gotEntries, "test-model")
}

func TestEncodeImageHTTP(t *testing.T) {
	var gotImage bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		file, _, err := r.FormFile("image")
		require.NoError(t, err)
		defer file.Close()
		data, err := io.ReadAll(file)
		require.NoError(t, err)
		gotImage = len(data) > 0
		_, _ = w.Write([]byte(`{"clip":[1,2,3,4],"imageHeight":10,"imageWidth":20}`))
	}))
	defer srv.Close()

	c := NewClient(Config{Enabled: true, URL: srv.URL})
	emb, err := c.EncodeImage(context.Background(), []byte("jpeg-bytes"), "ViT-B-32__openai")
	require.NoError(t, err)
	require.Len(t, emb, 4)
	assert.True(t, gotImage)
}

func TestDetectFacesHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		entries := r.FormValue("entries")
		assert.Contains(t, entries, `"facial-recognition"`)
		assert.Contains(t, entries, `"detection"`)
		assert.Contains(t, entries, `"recognition"`)
		assert.Contains(t, entries, "buffalo_l")
		_, _ = w.Write([]byte(`{
			"facial-recognition": [
				{"boundingBox":{"x1":5,"y1":6,"x2":50,"y2":60},"embedding":"[0.9]","score":0.88}
			],
			"imageHeight":100,
			"imageWidth":200
		}`))
	}))
	defer srv.Close()

	c := NewClient(Config{Enabled: true, URL: srv.URL, FaceModel: "buffalo_l", FaceMinScore: 0.7})
	result, err := c.DetectFaces(context.Background(), []byte("img"), "", 0)
	require.NoError(t, err)
	require.Len(t, result.Faces, 1)
	assert.Equal(t, int32(5), result.Faces[0].BoundingBox.X1)
	assert.Equal(t, 200, result.ImageWidth)
	assert.InDelta(t, 0.88, result.Faces[0].Score, 1e-6)
}

func TestPredictUnavailableStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("down"))
	}))
	defer srv.Close()

	c := NewClient(Config{Enabled: true, URL: srv.URL})
	_, err := c.EncodeText(context.Background(), "hello", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.True(t, strings.Contains(err.Error(), "503") || strings.Contains(err.Error(), "unavailable"))
}

func TestPing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ping", r.URL.Path)
		_, _ = w.Write([]byte("pong"))
	}))
	defer srv.Close()

	c := NewClient(Config{Enabled: true, URL: srv.URL})
	require.NoError(t, c.Ping(context.Background()))
}
