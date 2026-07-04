//go:build integration
// +build integration

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/denysvitali/immich-go-backend/internal/db/sqlc"
	"github.com/denysvitali/immich-go-backend/internal/db/testdb"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEnqueuer implements taskEnqueuer for tests.
type fakeEnqueuer struct {
	tasks []*asynq.Task
	opts  [][]asynq.Option
}

func (f *fakeEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.tasks = append(f.tasks, task)
	f.opts = append(f.opts, opts)
	return &asynq.TaskInfo{ID: uuid.NewString(), Queue: "normal"}, nil
}

func (f *fakeEnqueuer) Close() error { return nil }

// newTestService creates a Service backed by a real DB and a fake queue client.
func newTestService(t *testing.T, tdb *testdb.TestDB) *Service {
	t.Helper()
	log := logrus.New()
	log.SetLevel(logrus.FatalLevel)
	return &Service{
		client:     &fakeEnqueuer{},
		logger:     log,
		handlers:   make(map[string]func(context.Context, *asynq.Task) error),
		db:         tdb.Queries,
		maxRetries: 10,
	}
}

func TestIntegration_RecordJobFailure_SkipRetry(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	svc := newTestService(t, tdb)

	payload := ThumbnailGenerationPayload{AssetID: uuid.New().String()}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(string(JobTypeThumbnailGeneration), payloadBytes)
	baseErr := errors.New("unsupported: face detection requires an ML integration, which is not configured")
	wrappedErr := fmt.Errorf("%w", errors.Join(baseErr, asynq.SkipRetry))

	queue := "normal"
	retryCount := 0
	maxRetry := 10

	require.NoError(t, svc.recordJobFailure(ctx, task, queue, retryCount, maxRetry, wrappedErr))

	failures, err := tdb.Queries.ListJobFailures(ctx, sqlc.ListJobFailuresParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, failures, 1)

	failure := failures[0]
	assert.Equal(t, queue, failure.Queue)
	assert.Equal(t, string(JobTypeThumbnailGeneration), failure.JobType)
	assert.Equal(t, int32(maxRetry), failure.MaxRetries)
	assert.Equal(t, int32(retryCount), failure.RetriedCount)
	assert.Contains(t, failure.Error, "skip retry")
}

func TestIntegration_ListDeadLetterJobs(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	svc := newTestService(t, tdb)

	for i := 0; i < 3; i++ {
		payload := ThumbnailGenerationPayload{AssetID: uuid.New().String()}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		task := asynq.NewTask(string(JobTypeThumbnailGeneration), payloadBytes)
		require.NoError(t, svc.recordJobFailure(ctx, task, "normal", i, 10, asynq.SkipRetry))
		time.Sleep(10 * time.Millisecond)
	}

	jobs, err := svc.ListDeadLetterJobs(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, jobs, 3)

	// Should be ordered by failed_at DESC (newest first).
	for i := 0; i < len(jobs)-1; i++ {
		assert.True(t, jobs[i].FailedAt.After(jobs[i+1].FailedAt) || jobs[i].FailedAt.Equal(jobs[i+1].FailedAt))
	}
}

func TestIntegration_RetryDeadLetterJob(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	svc := newTestService(t, tdb)

	payload := ThumbnailGenerationPayload{AssetID: uuid.New().String()}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(string(JobTypeThumbnailGeneration), payloadBytes)
	require.NoError(t, svc.recordJobFailure(ctx, task, "normal", 0, 10, asynq.SkipRetry))

	failures, err := tdb.Queries.ListJobFailures(ctx, sqlc.ListJobFailuresParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, failures, 1)
	failureID := uuid.UUID(failures[0].ID.Bytes).String()

	require.NoError(t, svc.RetryDeadLetterJob(ctx, failureID))

	failures, err = tdb.Queries.ListJobFailures(ctx, sqlc.ListJobFailuresParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Empty(t, failures)

	enqueuer := svc.client.(*fakeEnqueuer)
	require.Len(t, enqueuer.tasks, 1)
	assert.Equal(t, string(JobTypeThumbnailGeneration), enqueuer.tasks[0].Type())
	assert.JSONEq(t, string(payloadBytes), string(enqueuer.tasks[0].Payload()))
}

func TestIntegration_RetryDeadLetterJob_InvalidID(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()
	svc := newTestService(t, tdb)

	err := svc.RetryDeadLetterJob(ctx, "not-a-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid job failure id")
}

func TestIntegration_UnsupportedJobError_LandsInDeadLetter(t *testing.T) {
	testdb.SkipIfNoDocker(t)

	tdb := testdb.SetupTestDB(t)
	ctx := context.Background()

	svc := newTestService(t, tdb)
	svc.RegisterHandler(JobTypeFaceDetection, func(_ context.Context, _ *asynq.Task) error {
		return unsupportedJobError("face detection requires an ML integration, which is not configured")
	})

	payload := FaceDetectionPayload{AssetID: uuid.New().String()}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(string(JobTypeFaceDetection), payloadBytes)
	err = svc.handlers[string(JobTypeFaceDetection)](ctx, task)
	require.Error(t, err)
	require.True(t, errors.Is(err, asynq.SkipRetry), "expected SkipRetry error")

	// Simulate what the asynq ErrorHandler would do after the handler returns.
	require.NoError(t, svc.recordJobFailure(ctx, task, "normal", 0, 10, err))

	failures, err := tdb.Queries.ListJobFailures(ctx, sqlc.ListJobFailuresParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, failures, 1)
	assert.Equal(t, string(JobTypeFaceDetection), failures[0].JobType)
	assert.Contains(t, failures[0].Error, "face detection requires an ML integration")
}
