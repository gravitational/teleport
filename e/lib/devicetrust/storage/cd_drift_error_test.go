package storage_test

import (
	"errors"
	"testing"

	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
)

func TestCollectedDataDriftError_Is(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "CollectedDataDriftError",
			err:  storage.NewCollectedDataDriftError("something bad"),
			want: true,
		},
		{
			name: "unrelated error",
			err:  errors.New("some other error"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cdErr := &storage.CollectedDataDriftError{}
			if got, want := cdErr.Is(test.err), test.want; got != want {
				t.Errorf("CollectedDataDriftError.Is=%v, want=%v", got, want)
			}
			if got, want := errors.Is(test.err, cdErr), test.want; got != want {
				t.Errorf("errors.Is=%v, want=%v", got, want)
			}
		})
	}
}
