package internal

import (
	"context"
	"errors"
	"testing"
)

type testArchiveStore struct {
	deletedID string
}

func (*testArchiveStore) UploadArchive(context.Context, string, string) (string, error) {
	return "", nil
}

func (store *testArchiveStore) DeleteArchive(_ context.Context, deploymentID string) error {
	store.deletedID = deploymentID
	return nil
}

type testBuildQueue struct {
	err      error
	queued   bool
	checkErr error
}

func (queue *testBuildQueue) EnqueueBuild(context.Context, string) error {
	return queue.err
}

func (queue *testBuildQueue) HasDeployment(context.Context, string) (bool, error) {
	return queue.queued, queue.checkErr
}

func TestEnqueueBuildCleansUpOnlyOnFailure(t *testing.T) {
	const deploymentID = "0123456789abcdef"
	for _, test := range []struct {
		name       string
		queueError error
		queued     bool
		checkError error
		wantError  bool
		wantDelete bool
	}{
		{name: "success"},
		{name: "definite queue failure", queueError: errors.New("Redis rejected push"), wantError: true, wantDelete: true},
		{name: "push accepted but response lost", queueError: errors.New("connection dropped"), queued: true},
		{name: "outcome unknown", queueError: errors.New("connection dropped"), checkError: errors.New("Redis unavailable"), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &testArchiveStore{}
			handler := NewDeployHandler(store, &testBuildQueue{err: test.queueError, queued: test.queued, checkErr: test.checkError})
			err := handler.enqueueBuild(context.Background(), deploymentID)
			if test.wantError && !errors.Is(err, test.queueError) {
				t.Fatalf("enqueueBuild() error = %v, want %v", err, test.queueError)
			}
			if !test.wantError && err != nil {
				t.Fatalf("enqueueBuild() error = %v, want nil", err)
			}
			if test.wantDelete && store.deletedID != deploymentID {
				t.Fatalf("deleted deployment ID = %q, want %q", store.deletedID, deploymentID)
			}
			if !test.wantDelete && store.deletedID != "" {
				t.Fatalf("unexpectedly deleted deployment ID %q", store.deletedID)
			}
		})
	}
}
