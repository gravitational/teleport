package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <filename>\n", os.Args[0])
	}

	in, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("could not open %v: %v", os.Args[1], err)
	}
	defer in.Close()

	out, err := os.Create(filepath.Base(os.Args[1]) + ".cast")
	if err != nil {
		log.Fatalf("could not open output file: %v", err)
	}
	defer func() {
		out.Close()
		log.Printf("wrote %v", out.Name())
	}()

	r := events.NewProtoReader(in)
	defer r.Close()

	enc := json.NewEncoder(out)

	wroteHeader := false
	var lastDelay float64
	var lastTime time.Time
	for {
		evt, err := r.Read(context.Background())
		if err == io.EOF {
			return
		} else if err != nil {
			log.Fatal(err)
		}

		// most events have a an absolute timestamp, but don't indicate
		// how far into the session we were
		elapsed := evt.GetTime().Sub(lastTime).Seconds()

		switch evt := evt.(type) {
		case *apievents.SessionStart:
			if !wroteHeader {
				enc.Encode(header(evt))
				wroteHeader = true
			}
			lastTime = evt.GetTime()
		case *apievents.SessionPrint:
			if !wroteHeader {
				log.Println("skipping print event, waiting for header")
				continue
			}

			lastDelay = float64(evt.DelayMilliseconds) / 1000.0
			lastTime = evt.GetTime()

			enc.Encode(outputEvent(evt))

		case *apievents.Resize:
			enc.Encode(resizeEvent(lastDelay+elapsed, evt))
			lastTime = evt.GetTime()
			lastDelay += elapsed

		case *apievents.SessionJoin:
			enc.Encode(joinMarker(lastDelay+elapsed, evt))
			lastTime = evt.GetTime()
			lastDelay += elapsed

		case *apievents.SessionLeave:
			enc.Encode(leaveMarker(lastDelay+elapsed, evt))
			lastTime = evt.GetTime()
			lastDelay += elapsed

		default:
			log.Printf("skipping %T event", evt)
			continue
		}

	}
}

// asciicast event codes:
// https://docs.asciinema.org/manual/asciicast/v2/#supported-event-codes
const (
	codeOutput = "o"
	codeInput  = "i"
	codeMarker = "m"
	codeResize = "r"
)

func outputEvent(evt *apievents.SessionPrint) [3]any {
	return [3]any{
		float64(evt.DelayMilliseconds) / 1000.0,
		codeOutput,
		string(evt.Data),
	}
}

func resizeEvent(time float64, evt *apievents.Resize) [3]any {
	return [3]any{
		time,
		codeResize,
		strings.ReplaceAll(evt.TerminalSize, ":", "x"),
	}
}

func joinMarker(time float64, evt *apievents.SessionJoin) [3]any {
	return [3]any{
		time,
		codeMarker,
		fmt.Sprintf("User %v joined the session", evt.User),
	}
}

func leaveMarker(time float64, evt *apievents.SessionLeave) [3]any {
	return [3]any{
		time,
		codeMarker,
		fmt.Sprintf("User %v left the session", evt.User),
	}
}

// header returns the data to be encoded in an asciicast v2 header
//
// See https://docs.asciinema.org/manual/asciicast/v2/#header
func header(start *apievents.SessionStart) map[string]any {
	var w, h int
	tp, err := session.UnmarshalTerminalParams(start.TerminalSize)
	if err == nil {
		w, h = tp.W, tp.H
	}

	return map[string]interface{}{
		// required header attributes:
		"version": 2,
		"width":   w,
		"height":  h,

		// optional header attributes:
		"timestamp": start.GetTime().Unix(),
		"command":   strings.Join(start.InitialCommand, " "),
		"title":     start.SessionID,
	}
}
