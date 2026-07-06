package storage

import (
	"bytes"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS3UploadInputSetsKnownContentLength(t *testing.T) {
	input := newS3UploadInput("bucket", "asset.jpg", bytes.NewReader(nil), 42, "image/jpeg")

	require.NotNil(t, input.ContentLength)
	assert.Equal(t, int64(42), aws.ToInt64(input.ContentLength))
}

func TestNewS3UploadInputOmitsUnknownContentLength(t *testing.T) {
	input := newS3UploadInput("bucket", "asset.jpg", bytes.NewReader(nil), -1, "image/jpeg")

	assert.Nil(t, input.ContentLength)
}
