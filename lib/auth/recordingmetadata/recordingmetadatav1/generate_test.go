package recordingmetadatav1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

func TestStream(t *testing.T) {
	// Clean up existing thumbnails directory before starting
	sessionDir := filepath.Join("thumbnails")
	if err := os.RemoveAll(sessionDir); err != nil {
		t.Logf("Warning: Failed to remove existing thumbnails directory: %v", err)
	}

	// Create fresh thumbnails directory
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		t.Fatalf("Failed to create thumbnails directory: %v", err)
	}

	capture, err := newDesktopThumbnailCapture(1*time.Second, 1000)
	require.NoError(t, err)

	streamStart := time.Now()
	evts, errs := streamSessionEvents(context.Background(), 0)

	var allEvents []apievents.AuditEvent

Loop:
	for {
		select {
		case evt, ok := <-evts:
			if !ok {
				evts = nil
				if errs == nil {
					break Loop
				}
				continue
			}
			switch e := evt.(type) {
			case *apievents.DesktopRecording:
				err := capture.handleEvent(e)
				require.NoError(t, err)
				allEvents = append(allEvents, evt)
			case *apievents.WindowsDesktopSessionEnd:
				fmt.Println("Received session end event")
				allEvents = append(allEvents, evt)
				break Loop
			}
		case err := <-errs:
			if err == nil || errors.Is(err, io.EOF) {
				errs = nil
				if evts == nil {
					break Loop
				}
				continue
			}
			t.Fatalf("error streaming events: %v", err)
		}
	}

	//// write all events to events.json
	//streamedEventsFile := "events.json"
	//f, err := os.Create(streamedEventsFile)
	//data, err := json.Marshal(allEvents)
	//require.NoError(t, err)
	//_, err = f.Write(data)
	//require.NoError(t, err)
	//err = f.Close()
	//require.NoError(t, err)

	t.Logf("Streamed events in %s", time.Since(streamStart))

	start := time.Now()

	opts := thumbnailOptions{
		width:      2400, // Higher resolution for better quality
		height:     1000, // 720p HD resolution
		zoomFactor: 1.0,  // No zoom to show more of the desktop
	}

	fmt.Println("snapshots:", len(capture.snapshots))

	generatedCount := 0
	for i := range capture.snapshots {
		thumbnail, err := capture.GenerateThumbnail(i, opts)
		if err != nil {
			t.Logf("Warning: Failed to generate thumbnail %d: %v", i, err)
			continue
		}
		if thumbnail == nil {
			t.Logf("Skipping empty thumbnail %d", i)
			continue
		}

		// Write thumbnail to file
		filename := filepath.Join(sessionDir, fmt.Sprintf("thumbnail_%04d.png", i))
		err = os.WriteFile(filename, thumbnail, 0644)
		require.NoError(t, err)
		generatedCount++

		// Log every 10th thumbnail for progress
		if i%10 == 0 {
			t.Logf("Generated thumbnail %d/%d: %s", i, len(capture.snapshots), filename)
		}
	}

	t.Logf("Session complete: Generated %d thumbnails in %s",
		generatedCount, sessionDir)

	// Verify we generated at least some thumbnails
	require.Greater(t, generatedCount, 0, "Should have generated at least one thumbnail")

	t.Logf("Thumbnail generation took %s", time.Since(start))
}

func streamSessionEvents(
	ctx context.Context,
	startIndex int64,
) (chan apievents.AuditEvent, chan error) {
	evts := make(chan apievents.AuditEvent)
	errs := make(chan error, 1)

	go func() {
		f, err := os.Open("recording.tar")
		if err != nil {
			errs <- trace.ConvertSystemError(err)
			return
		}
		defer f.Close()

		pr := events.NewProtoReader(f, nil)

		for i := int64(0); ; i++ {
			evt, err := pr.Read(ctx)
			if err != nil {
				errs <- trace.Wrap(err)
				return
			}

			if i >= startIndex {
				select {
				case evts <- evt:
				case <-ctx.Done():
					errs <- trace.Wrap(ctx.Err())
					return
				}
			}
		}
	}()

	return evts, errs
}
