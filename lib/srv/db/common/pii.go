/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"context"
	"log/slog"
	"sort"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/comprehend"
	comprehendtypes "github.com/aws/aws-sdk-go-v2/service/comprehend/types"
	"github.com/gravitational/trace"
)

const (
	// comprehendMaxBytes is the maximum UTF-8 byte length accepted by
	// DetectPiiEntities. Queries longer than this are truncated.
	comprehendMaxBytes = 5000

	// piiConfidenceThreshold is the minimum Comprehend score required for an
	// entity to be reported. Entities below this threshold are logged at debug
	// level and discarded.
	piiConfidenceThreshold = float32(0.70)
)

// piiSpan is an internal representation of a detected PII entity with its
// byte offsets. The original value is intentionally not stored.
type piiSpan struct {
	entityType  string
	beginOffset int
	endOffset   int
}

// PIIDetector calls AWS Comprehend to identify PII entity types in database
// queries. It is safe for concurrent use.
type PIIDetector struct {
	client *comprehend.Client
	logger *slog.Logger
}

// NewPIIDetector creates a PIIDetector using the default AWS credential chain
// (env vars, ~/.aws/credentials, EC2/ECS instance role, etc.).
func NewPIIDetector(ctx context.Context) (*PIIDetector, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &PIIDetector{
		client: comprehend.NewFromConfig(cfg),
		logger: slog.Default(),
	}, nil
}

// NewPIIDetectorFromConfig creates a PIIDetector from a pre-built aws.Config.
func NewPIIDetectorFromConfig(cfg aws.Config) *PIIDetector {
	return &PIIDetector{
		client: comprehend.NewFromConfig(cfg),
		logger: slog.Default(),
	}
}

// Detect runs PII detection on the provided query string and returns the
// entity type names (e.g. ["SSN", "EMAIL"]) that exceed the confidence
// threshold. The query value itself is never stored in the result.
// Returns nil when no qualifying entities are found.
func (d *PIIDetector) Detect(ctx context.Context, query string) ([]string, error) {
	spans, err := d.detect(ctx, query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	types := make([]string, 0, len(spans))
	for _, s := range spans {
		types = append(types, s.entityType)
	}
	return types, nil
}

// Redact returns a copy of query with each PII span replaced by its entity
// type label (e.g. [NAME], [ADDRESS]) and the list of detected entity types.
// The original PII values are never stored anywhere in this process.
func (d *PIIDetector) Redact(ctx context.Context, query string) (string, []string, error) {
	spans, err := d.detect(ctx, query)
	if err != nil {
		return query, nil, trace.Wrap(err)
	}
	if len(spans) == 0 {
		return query, nil, nil
	}

	types := make([]string, 0, len(spans))
	for _, s := range spans {
		types = append(types, s.entityType)
	}

	return redactSpans(query, spans), types, nil
}

// detect calls Comprehend and returns spans that exceed the confidence
// threshold. It is the shared implementation for Detect and Redact.
func (d *PIIDetector) detect(ctx context.Context, query string) ([]piiSpan, error) {
	text := truncateToBytes(query, comprehendMaxBytes)

	resp, err := d.client.DetectPiiEntities(ctx, &comprehend.DetectPiiEntitiesInput{
		Text:         aws.String(text),
		LanguageCode: comprehendtypes.LanguageCodeEn,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var spans []piiSpan
	for _, e := range resp.Entities {
		score := float32(0)
		if e.Score != nil {
			score = *e.Score
		}

		if score < piiConfidenceThreshold {
			d.logger.DebugContext(ctx, "PII entity below confidence threshold, skipping",
				"type", e.Type,
				"score", score,
				"threshold", piiConfidenceThreshold,
			)
			continue
		}

		if e.BeginOffset == nil || e.EndOffset == nil {
			continue
		}
		spans = append(spans, piiSpan{
			entityType:  string(e.Type),
			beginOffset: int(*e.BeginOffset),
			endOffset:   int(*e.EndOffset),
		})
	}
	return spans, nil
}

// redactSpans replaces each span in text with "[TYPE]". Spans are processed
// right-to-left so earlier byte offsets remain valid after each substitution.
func redactSpans(text string, spans []piiSpan) string {
	// Sort descending by begin offset so substitutions don't shift subsequent offsets.
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].beginOffset > spans[j].beginOffset
	})

	b := []byte(text)
	for _, s := range spans {
		if s.beginOffset < 0 || s.endOffset > len(b) || s.beginOffset >= s.endOffset {
			continue
		}
		label := "<REDACTED:" + s.entityType + ">"
		b = append(b[:s.beginOffset], append([]byte(label), b[s.endOffset:]...)...)
	}
	return string(b)
}

// truncateToBytes returns s truncated so that its UTF-8 byte length does not
// exceed maxBytes, cutting only on rune boundaries.
func truncateToBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i]
		}
	}
	return ""
}
