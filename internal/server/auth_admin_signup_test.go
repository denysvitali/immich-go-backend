//go:build integration
// +build integration

package server

import (
	"context"
	"testing"

	"github.com/denysvitali/immich-go-backend/internal/auth"
	"github.com/denysvitali/immich-go-backend/internal/config"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	immichv1 "github.com/denysvitali/immich-go-backend/internal/proto/gen/immich/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestServerAdminSignUpReturnsInvalidArgumentForShortPassword verifies that a
// password failing complexity checks surfaces as codes.InvalidArgument with
// the validation reason — not codes.Internal. The Immich web client shows
// the gRPC message to the user during onboarding, so a generic
// "admin registration failed" hides the real cause (password too short,
// missing uppercase, etc.) and makes it look like the backend is broken.
//
// Regression test for the production bug where every AdminSignUp failure was
// funnelled through SanitizedInternal, including caller-side validation
// failures that the user could fix.
func TestServerAdminSignUpReturnsInvalidArgumentForShortPassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           1,
		JWTRefreshExpiry:    1,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
		// Keep the other complexity flags off so this test only exercises the
		// length check, matching the production bug repro (1-char password).
		PasswordRequireUppercase: false,
		PasswordRequireLowercase: false,
		PasswordRequireNumbers:   false,
		PasswordRequireSymbols:   false,
	}

	srv := &Server{
		authService: auth.NewService(cfg, tdb.Queries),
	}

	_, err := srv.AdminSignUp(ctx, &immichv1.AdminSignUpRequest{
		Email:    "admin@test.com",
		Password: "x",
		Name:     "Admin",
	})
	require.Error(t, err, "AdminSignUp with 1-char password must fail")

	grpcStatus, ok := status.FromError(err)
	require.True(t, ok, "error must be a gRPC status error")
	assert.Equal(t, codes.InvalidArgument, grpcStatus.Code(),
		"short password must surface as InvalidArgument, not Internal")
	assert.Contains(t, grpcStatus.Message(), "8 characters",
		"underlying validation message must be preserved so the UI can show why")
}

// TestServerAdminSignUpSucceedsWithValidPassword is the happy path: a
// password meeting complexity requirements should produce a 200 with an
// access token. Guards against the AdminSignUp handler accidentally
// rejecting valid input after the InvalidArgument refactor.
func TestServerAdminSignUpSucceedsWithValidPassword(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	cfg := config.AuthConfig{
		JWTSecret:           "test-secret-key-for-testing-only-needs-32-chars",
		JWTExpiry:           1,
		JWTRefreshExpiry:    1,
		RegistrationEnabled: true,
		PasswordMinLength:   8,
	}

	srv := &Server{
		authService: auth.NewService(cfg, tdb.Queries),
	}

	resp, err := srv.AdminSignUp(ctx, &immichv1.AdminSignUpRequest{
		Email:    "admin-success@test.com",
		Password: "validpassword123",
		Name:     "Admin",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.GetAccessToken())
	assert.Equal(t, "admin-success@test.com", resp.GetUserEmail())
	assert.True(t, resp.GetIsAdmin(), "first admin signup must produce an admin user")
}
