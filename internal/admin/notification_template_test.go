package admin

import (
	"context"
	"errors"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTemplateTestService(t *testing.T) *Service {
	t.Helper()

	service, err := NewService(nil, &config.Config{}, nil)
	require.NoError(t, err)
	return service
}

func TestRenderNotificationTemplateUsesCustomTemplateData(t *testing.T) {
	service := newTemplateTestService(t)

	resp, err := service.RenderNotificationTemplate(
		context.Background(),
		"welcome",
		"Hello {displayName}, log in at {baseUrl}. Unknown {missing}",
	)
	require.NoError(t, err)

	assert.Equal(t, "welcome", resp.Name)
	assert.Contains(t, resp.HTML, "Hello John Doe, log in at https://demo.immich.app.")
	assert.Contains(t, resp.HTML, "Unknown {missing}")
}

func TestRenderNotificationTemplateEscapesCustomTemplate(t *testing.T) {
	service := newTemplateTestService(t)

	resp, err := service.RenderNotificationTemplate(
		context.Background(),
		"album-invite",
		"Album {albumName}<script>alert(1)</script>",
	)
	require.NoError(t, err)

	assert.Equal(t, "album-invite", resp.Name)
	assert.Contains(t, resp.HTML, "Album John Doe&#39;s Favorites")
	assert.Contains(t, resp.HTML, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.NotContains(t, resp.HTML, "<script>")
}

func TestRenderNotificationTemplateUnknownNameMatchesUpstreamEmptyPreview(t *testing.T) {
	service := newTemplateTestService(t)

	resp, err := service.RenderNotificationTemplate(context.Background(), "unknown", "ignored")
	require.NoError(t, err)

	assert.Equal(t, "unknown", resp.Name)
	assert.Empty(t, resp.HTML)
}

func TestRenderNotificationTemplateServerReturnsUpstreamShape(t *testing.T) {
	service := newTemplateTestService(t)
	server := NewServer(service, nil)

	resp, err := server.RenderNotificationTemplate(adminContext(), &immichv1.RenderNotificationTemplateRequest{
		Name:     "album-update",
		Template: "Album {albumName} for {recipientName}",
	})
	require.NoError(t, err)

	assert.Equal(t, "album-update", resp.GetName())
	assert.Contains(t, resp.GetHtml(), "Album Favorite Photos for Jane Doe")
}

func TestTestEmailNotificationSendsRenderedEmailToCurrentAdmin(t *testing.T) {
	service := newTemplateTestService(t)
	sender := &fakeEmailSender{}
	service.email = sender

	ctx := auth.WithUser(context.Background(), auth.UserInfo{
		Email:   "admin@example.com",
		Name:    "Admin User",
		IsAdmin: true,
	})
	resp, err := service.TestEmailNotification(ctx, TestEmailNotificationRequest{
		SMTP: SMTPConfig{
			From:    "Immich <noreply@example.com>",
			ReplyTo: "reply@example.com",
			Transport: SMTPTransport{
				Host:       "smtp.example.com",
				Port:       2465,
				Username:   "smtp-user",
				Password:   "smtp-password",
				IgnoreCert: true,
				Secure:     true,
			},
		},
		Template: "Hello {displayName}, open {baseUrl}. Unknown {missing}",
	})
	require.NoError(t, err)

	assert.Contains(t, resp.MessageID, "@immich-go>")
	assert.Equal(t, 1, sender.verifyCalls)
	assert.Equal(t, 1, sender.sendCalls)
	assert.Equal(t, "smtp.example.com", sender.verified.Host)
	assert.True(t, sender.verified.Secure)
	assert.Equal(t, "Immich <noreply@example.com>", sender.sent.From)
	assert.Equal(t, "reply@example.com", sender.sent.ReplyTo)
	assert.Equal(t, "admin@example.com", sender.sent.To)
	assert.Equal(t, "Test email from Immich", sender.sent.Subject)
	assert.Equal(t, resp.MessageID, sender.sent.MessageID)
	assert.Contains(t, sender.sent.Text, "Hello Admin User, open https://demo.immich.app.")
	assert.Contains(t, sender.sent.Text, "Unknown {missing}")
	assert.Contains(t, sender.sent.HTML, "Hello Admin User")
}

func TestTestEmailNotificationRequiresValidSMTPConfig(t *testing.T) {
	service := newTemplateTestService(t)
	service.email = &fakeEmailSender{}
	ctx := auth.WithUser(context.Background(), auth.UserInfo{
		Email:   "admin@example.com",
		Name:    "Admin User",
		IsAdmin: true,
	})

	_, err := service.TestEmailNotification(ctx, TestEmailNotificationRequest{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidSMTPConfig)
}

func TestTestEmailNotificationServerReturnsMessageIDShape(t *testing.T) {
	service := newTemplateTestService(t)
	service.email = &fakeEmailSender{}
	server := NewServer(service, nil)
	ctx := auth.WithUser(context.Background(), auth.UserInfo{
		Email:   "admin@example.com",
		Name:    "Admin User",
		IsAdmin: true,
	})

	resp, err := server.TestEmailNotification(ctx, &immichv1.TestEmailNotificationRequest{
		Smtp: &immichv1.SystemConfigNotificationsSmtpDto{
			From:    "Immich <noreply@example.com>",
			ReplyTo: "reply@example.com",
			Transport: &immichv1.SystemConfigSmtpTransportDto{
				Host:       "smtp.example.com",
				Port:       587,
				Username:   "smtp-user",
				Password:   "smtp-password",
				IgnoreCert: true,
				Secure:     true,
			},
		},
		Template: "Hello {displayName}",
	})
	require.NoError(t, err)

	assert.NotEmpty(t, resp.GetMessageId())
}

func TestTestEmailNotificationServerReturnsInvalidArgumentForSMTPFailure(t *testing.T) {
	service := newTemplateTestService(t)
	service.email = &fakeEmailSender{verifyErr: errors.New("connection refused")}
	server := NewServer(service, nil)

	_, err := server.TestEmailNotification(adminContext(), &immichv1.TestEmailNotificationRequest{
		Smtp: &immichv1.SystemConfigNotificationsSmtpDto{
			From: "noreply@example.com",
			Transport: &immichv1.SystemConfigSmtpTransportDto{
				Host: "smtp.example.com",
				Port: 587,
			},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "InvalidArgument")
	assert.Contains(t, err.Error(), "failed to verify SMTP configuration")
}

type fakeEmailSender struct {
	verifyCalls int
	sendCalls   int
	verified    SMTPTransport
	sent        emailMessage
	verifyErr   error
	sendErr     error
}

func (f *fakeEmailSender) Verify(_ context.Context, transport SMTPTransport) error {
	f.verifyCalls++
	f.verified = transport
	return f.verifyErr
}

func (f *fakeEmailSender) Send(_ context.Context, message emailMessage) error {
	f.sendCalls++
	f.sent = message
	return f.sendErr
}
