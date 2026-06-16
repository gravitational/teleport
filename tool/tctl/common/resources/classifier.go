// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// classifierCollection is a collection of Classifier resources that can be
// written to an io.Writer in a human-readable format.
type classifierCollection []*summarizerv1.Classifier

func (c classifierCollection) Resources() []types.Resource {
	out := make([]types.Resource, 0, len(c))
	for _, item := range c {
		out = append(out, classifierResource{
			Resource:   types.ProtoResource153ToLegacy(item),
			classifier: item,
		})
	}
	return out
}

// classifierResource renders a Classifier with the tri-state ClassifierActionMode
// fields (emit_audit_event, flag_for_review) presented as booleans in "tctl get"
// YAML/JSON output.
type classifierResource struct {
	types.Resource
	classifier *summarizerv1.Classifier
}

// MarshalJSON renders the wrapped Classifier, converting the tri-state action
// mode enums into booleans. It mirrors the protojson options used by the generic
// RFD 153 adapter so that every other field is encoded identically.
func (r classifierResource) MarshalJSON() ([]byte, error) {
	data, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(r.classifier)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return classifierActionsToBool(data)
}

func (c classifierCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "Description", "Kinds"}
	var rows [][]string
	for _, item := range c {
		meta := item.GetMetadata()
		spec := item.GetSpec()
		rows = append(rows, []string{
			meta.GetName(),
			meta.GetDescription(),
			strings.Join(spec.GetKinds(), ", "),
		})
	}
	var t asciitable.Table
	if verbose {
		t = asciitable.MakeTable(headers, rows...)
	} else {
		t = asciitable.MakeTableWithTruncatedColumn(headers, rows, "Description")
	}

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func classifierHandler() Handler {
	return Handler{
		getHandler:    getClassifiers,
		createHandler: createClassifier,
		updateHandler: updateClassifier,
		deleteHandler: deleteClassifier,
		singleton:     false,
		mfaRequired:   false,
		description:   "Specifies a classifier applied to sessions during summarization",
	}
}

func createClassifier(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts,
) error {
	rawJSON, err := classifierActionsFromBool(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	classifier, err := services.UnmarshalProtoResource[*summarizerv1.Classifier](
		rawJSON, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	sclt := clt.SummarizerServiceClient()
	if opts.Force {
		req := summarizerv1.UpsertClassifierRequest_builder{
			Classifier: classifier,
		}.Build()
		_, err = sclt.UpsertClassifier(ctx, req)
	} else {
		req := summarizerv1.CreateClassifierRequest_builder{
			Classifier: classifier,
		}.Build()
		_, err = sclt.CreateClassifier(ctx, req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("classifier %q has been created\n", classifier.GetMetadata().GetName())
	return nil
}

func updateClassifier(
	ctx context.Context, clt *authclient.Client, raw services.UnknownResource, opts CreateOpts,
) error {
	rawJSON, err := classifierActionsFromBool(raw.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	classifier, err := services.UnmarshalProtoResource[*summarizerv1.Classifier](
		rawJSON, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	req := summarizerv1.UpdateClassifierRequest_builder{
		Classifier: classifier,
	}.Build()
	if _, err := clt.SummarizerServiceClient().UpdateClassifier(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("classifier %q has been updated\n", classifier.GetMetadata().GetName())
	return nil
}

func getClassifiers(
	ctx context.Context, clt *authclient.Client, ref services.Ref, opts GetOpts,
) (Collection, error) {
	ssclt := clt.SummarizerServiceClient()
	if ref.Name != "" {
		req := summarizerv1.GetClassifierRequest_builder{
			Name: ref.Name,
		}.Build()
		res, err := ssclt.GetClassifier(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return classifierCollection{res.GetClassifier()}, nil
	}

	items, err := stream.Collect(clientutils.Resources(
		ctx,
		func(
			ctx context.Context, _ int, nextPageToken string,
		) ([]*summarizerv1.Classifier, string, error) {
			resp, err := ssclt.ListClassifiers(
				ctx,
				summarizerv1.ListClassifiersRequest_builder{
					PageToken: nextPageToken,
				}.Build())
			return resp.GetClassifiers(), resp.GetNextPageToken(), trace.Wrap(err)
		}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return classifierCollection(items), nil
}

func deleteClassifier(
	ctx context.Context, clt *authclient.Client, ref services.Ref,
) error {
	req := summarizerv1.DeleteClassifierRequest_builder{
		Name: ref.Name,
	}.Build()
	if _, err := clt.SummarizerServiceClient().DeleteClassifier(ctx, req); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("classifier %q has been deleted\n", ref.Name)
	return nil
}

const (
	classifierActionModeEnabled  = "CLASSIFIER_ACTION_MODE_ENABLED"
	classifierActionModeDisabled = "CLASSIFIER_ACTION_MODE_DISABLED"
)

// classifierActionBoolFields are the spec.actions ClassifierActionMode toggles
// that tctl renders as booleans. risk_level_floor is a level rather than a
// toggle, so it is intentionally left as its enum value.
var classifierActionBoolFields = []string{"emit_audit_event", "flag_for_review"}

// classifierActionsToBool rewrites the ClassifierActionMode fields of a
// protojson-encoded Classifier from their enum string form into booleans.
// CLASSIFIER_ACTION_MODE_UNSPECIFIED is omitted by protojson, so it stays absent.
func classifierActionsToBool(data []byte) ([]byte, error) {
	return rewriteClassifierActions(data, func(v any) (any, bool) {
		switch v {
		case classifierActionModeEnabled:
			return true, true
		case classifierActionModeDisabled:
			return false, true
		default:
			return nil, false
		}
	})
}

// classifierActionsFromBool rewrites boolean ClassifierActionMode fields of a
// JSON-encoded Classifier into their enum string form so protojson can decode
// them. Non-boolean values (enum strings or numbers) are left untouched so
// existing manifests keep working.
func classifierActionsFromBool(data []byte) ([]byte, error) {
	return rewriteClassifierActions(data, func(v any) (any, bool) {
		b, ok := v.(bool)
		if !ok {
			return nil, false
		}
		if b {
			return classifierActionModeEnabled, true
		}
		return classifierActionModeDisabled, true
	})
}

// rewriteClassifierActions applies convert to each ClassifierActionMode field
// under spec.actions, replacing a value only when convert reports a change. The
// input and output are JSON. If the document has no spec.actions, or no fields
// change, the original bytes are returned unmodified.
func rewriteClassifierActions(data []byte, convert func(any) (any, bool)) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, trace.Wrap(err)
	}
	spec, ok := root["spec"].(map[string]any)
	if !ok {
		return data, nil
	}
	actions, ok := spec["actions"].(map[string]any)
	if !ok {
		return data, nil
	}
	changed := false
	for _, field := range classifierActionBoolFields {
		v, ok := actions[field]
		if !ok {
			continue
		}
		if nv, ok := convert(v); ok {
			actions[field] = nv
			changed = true
		}
	}
	if !changed {
		return data, nil
	}
	out, err := json.Marshal(root)
	return out, trace.Wrap(err)
}
