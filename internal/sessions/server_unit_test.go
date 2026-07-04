package sessions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionToProto(t *testing.T) {
	createdAt := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)

	session := &Session{
		ID:                 "session-id",
		UserID:             "user-id",
		DeviceType:         "WEB",
		DeviceOS:           "Linux",
		CreatedAt:          createdAt,
		UpdatedAt:          createdAt.Add(time.Minute),
		ExpiresAt:          createdAt.Add(24 * time.Hour),
		IsPendingSyncReset: true,
		AppVersion:         "1.2.3",
	}

	proto := sessionToProto(session, true)

	assert.Equal(t, "session-id", proto.Id)
	assert.Equal(t, "user-id", proto.UserId)
	assert.Equal(t, "WEB", proto.DeviceType)
	assert.Equal(t, "Linux", proto.DeviceOs)
	assert.True(t, proto.Current)
	assert.True(t, proto.IsPendingSyncReset)
	if assert.NotNil(t, proto.AppVersion) {
		assert.Equal(t, "1.2.3", *proto.AppVersion)
	}
	if assert.NotNil(t, proto.ExpiresAt) {
		assert.Equal(t, session.ExpiresAt.Unix(), proto.ExpiresAt.AsTime().Unix())
	}
}

func TestSessionToProtoOmitsEmptyOptionalFields(t *testing.T) {
	session := &Session{
		ID:        "session-id",
		UserID:    "user-id",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	proto := sessionToProto(session, false)

	assert.False(t, proto.Current)
	assert.False(t, proto.IsPendingSyncReset)
	assert.Nil(t, proto.AppVersion)
	assert.Nil(t, proto.ExpiresAt)
}
