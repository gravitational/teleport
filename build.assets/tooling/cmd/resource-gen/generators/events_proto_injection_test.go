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

package generators

import (
	"strings"
	"testing"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/stretchr/testify/require"
)

func TestNeededEventMessages(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "cookie",
			KindPascal: "Cookie",
			Operations: spec.OperationSet{Create: true, Update: true, Delete: true, Get: true, List: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "CO"},
		},
	}
	msgs := neededEventMessages(specs)

	require.Len(t, msgs, 3)
	require.Equal(t, "CookieCreate", msgs[0].Name)
	require.Equal(t, "cookie", msgs[0].Lower)
	require.Equal(t, "a", msgs[0].Article)
	require.Equal(t, "created", msgs[0].OpPastTense)

	require.Equal(t, "CookieDelete", msgs[1].Name)
	require.Equal(t, "deleted", msgs[1].OpPastTense)

	require.Equal(t, "CookieUpdate", msgs[2].Name)
	require.Equal(t, "updated", msgs[2].OpPastTense)
}

func TestNeededEventMessagesEmitOnGet(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "gadget",
			KindPascal: "Gadget",
			Operations: spec.OperationSet{Create: true, Update: true, Delete: true, Get: true, List: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, EmitOnGet: true, CodePrefix: "GA"},
		},
	}
	msgs := neededEventMessages(specs)

	require.Len(t, msgs, 4)
	names := make([]string, len(msgs))
	for i, m := range msgs {
		names[i] = m.Name
	}
	require.Equal(t, []string{"GadgetCreate", "GadgetDelete", "GadgetGet", "GadgetUpdate"}, names)

	// Check that GadgetGet has the right past tense.
	for _, m := range msgs {
		if m.Name == "GadgetGet" {
			require.Equal(t, "read", m.OpPastTense)
		}
	}
}

func TestNeededEventMessagesSkipsDisabledOps(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "widget",
			KindPascal: "Widget",
			Operations: spec.OperationSet{Create: true, Get: true, List: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "WI"},
		},
	}
	msgs := neededEventMessages(specs)

	// Update and Delete are disabled in operations, so only Create should be emitted.
	require.Len(t, msgs, 1)
	require.Equal(t, "WidgetCreate", msgs[0].Name)
}

func TestNeededEventMessagesMultipleSpecs(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "cookie",
			KindPascal: "Cookie",
			Operations: spec.OperationSet{Create: true, Delete: true, Get: true, List: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnDelete: true, CodePrefix: "CO"},
		},
		{
			Kind:       "beam",
			KindPascal: "Beam",
			Operations: spec.OperationSet{Create: true, Get: true, List: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, CodePrefix: "BE"},
		},
	}
	msgs := neededEventMessages(specs)

	// Should be sorted alphabetically: BeamCreate, CookieCreate, CookieDelete
	require.Len(t, msgs, 3)
	require.Equal(t, "BeamCreate", msgs[0].Name)
	require.Equal(t, "CookieCreate", msgs[1].Name)
	require.Equal(t, "CookieDelete", msgs[2].Name)
}

func TestInsertOneOfEntries(t *testing.T) {
	content := `message OneOf {
  oneof Event {
    events.Existing Existing = 1;
  }
}`
	entries := []eventMessageSpec{
		{Name: "FooCreate", Article: "a", FieldNum: 2},
		{Name: "FooDelete", Article: "a", FieldNum: 3},
	}
	result, err := insertOneOfEntries(content, entries)
	require.NoError(t, err)

	require.Contains(t, result, "events.FooCreate FooCreate = 2;")
	require.Contains(t, result, "events.FooDelete FooDelete = 3;")

	// Verify the existing entry is still there.
	require.Contains(t, result, "events.Existing Existing = 1;")

	// Verify closing braces are still present.
	require.True(t, strings.HasSuffix(strings.TrimSpace(result), "}"))
}

func TestInsertOneOfEntriesPreservesIndentation(t *testing.T) {
	content := `message OneOf {
  oneof Event {
    events.Existing Existing = 1;
  }
}`
	entries := []eventMessageSpec{
		{Name: "BarCreate", Article: "a", FieldNum: 2},
	}
	result, err := insertOneOfEntries(content, entries)
	require.NoError(t, err)

	// The new entry should have 4-space indentation (matching the existing entries).
	require.Contains(t, result, "    events.BarCreate BarCreate = 2;\n")
}

func TestInsertOneOfEntriesMissingOneof(t *testing.T) {
	content := `message OneOf {
  // no oneof here
}`
	entries := []eventMessageSpec{
		{Name: "FooCreate", Article: "a", FieldNum: 2},
	}
	_, err := insertOneOfEntries(content, entries)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not find 'oneof Event {'")
}

func TestAppendEventMessages(t *testing.T) {
	content := "syntax = \"proto3\";\n\nmessage Existing {}\n"
	msgs := []eventMessageSpec{
		{Name: "FooCreate", Lower: "foo", Article: "a", OpPastTense: "created"},
	}
	result := appendEventMessages(content, msgs)

	require.Contains(t, result, "// FooCreate is emitted when a foo resource is created.")
	require.Contains(t, result, "message FooCreate {")
	require.Contains(t, result, "Metadata Metadata = 1")
	require.Contains(t, result, "ResourceMetadata Resource = 2")
	require.Contains(t, result, "UserMetadata User = 3")
	require.Contains(t, result, "ConnectionMetadata Connection = 4")
	require.Contains(t, result, "Status Status = 5")

	// Ensure the original content is preserved.
	require.Contains(t, result, "message Existing {}")
}

func TestAppendEventMessagesMultiple(t *testing.T) {
	content := "// proto file\n"
	msgs := []eventMessageSpec{
		{Name: "FooCreate", Lower: "foo", Article: "a", OpPastTense: "created"},
		{Name: "FooDelete", Lower: "foo", Article: "a", OpPastTense: "deleted"},
	}
	result := appendEventMessages(content, msgs)

	require.Contains(t, result, "message FooCreate {")
	require.Contains(t, result, "message FooDelete {")

	// FooCreate should come before FooDelete.
	createIdx := strings.Index(result, "message FooCreate {")
	deleteIdx := strings.Index(result, "message FooDelete {")
	require.Less(t, createIdx, deleteIdx)
}

// --- Watch entry injection tests ---

func TestNeededWatchEntries(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "cookie",
			KindPascal:  "Cookie",
			ServiceName: "teleport.cookie.v1.CookieService",
			Cache:       spec.CacheConfig{Enabled: true},
		},
		{
			Kind:        "webhook",
			KindPascal:  "Webhook",
			ServiceName: "teleport.webhook.v1.WebhookService",
			Cache:       spec.CacheConfig{Enabled: true},
		},
	}
	entries := neededWatchEntries(specs)

	require.Len(t, entries, 2)
	// Sorted by Lower: cookie < webhook
	require.Equal(t, "Cookie", entries[0].Kind)
	require.Equal(t, "cookie", entries[0].Lower)
	require.Equal(t, "teleport/cookie/v1/cookie.proto", entries[0].ProtoImport)
	require.Equal(t, "teleport.cookie.v1.Cookie", entries[0].ProtoType)

	require.Equal(t, "Webhook", entries[1].Kind)
	require.Equal(t, "teleport/webhook/v1/webhook.proto", entries[1].ProtoImport)
	require.Equal(t, "teleport.webhook.v1.Webhook", entries[1].ProtoType)
}

func TestNeededWatchEntriesSkipsNonCache(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "no_cache",
			KindPascal:  "NoCache",
			ServiceName: "teleport.no_cache.v1.NoCacheService",
			Cache:       spec.CacheConfig{Enabled: false},
		},
	}
	// neededWatchEntries takes specs that are already filtered, but let's
	// test the function directly with non-cache resources; it should still
	// produce entries (filtering is done by InjectEventProtoWatch).
	entries := neededWatchEntries(specs)
	require.Len(t, entries, 1)
}

func TestInsertWatchImports(t *testing.T) {
	content := `syntax = "proto3";

import "teleport/accesslist/v1/accesslist.proto";
import "teleport/crownjewel/v1/crownjewel.proto";
import "teleport/workloadcluster/v1/workloadcluster.proto";

option go_package = "test";
`
	entries := []watchEntry{
		{Kind: "Cookie", ProtoImport: "teleport/cookie/v1/cookie.proto"},
	}
	result := insertWatchImports(content, entries)

	require.Contains(t, result, "import \"teleport/cookie/v1/cookie.proto\";")

	// Verify alphabetical order: cookie should be between accesslist and crownjewel.
	cookieIdx := strings.Index(result, "teleport/cookie/v1/cookie.proto")
	accesslistIdx := strings.Index(result, "teleport/accesslist/v1/accesslist.proto")
	crownjewelIdx := strings.Index(result, "teleport/crownjewel/v1/crownjewel.proto")
	require.Greater(t, cookieIdx, accesslistIdx)
	require.Less(t, cookieIdx, crownjewelIdx)
}

func TestInsertWatchImportsAlreadyExists(t *testing.T) {
	content := `import "teleport/cookie/v1/cookie.proto";
import "teleport/webhook/v1/webhook.proto";
`
	entries := []watchEntry{
		{Kind: "Cookie", ProtoImport: "teleport/cookie/v1/cookie.proto"},
	}
	result := insertWatchImports(content, entries)

	// Should not duplicate the import.
	require.Equal(t, 1, strings.Count(result, "teleport/cookie/v1/cookie.proto"))
}

func TestInsertWatchOneOfEntries(t *testing.T) {
	content := `message Event {
  Operation Type = 1;
  oneof Resource {
    types.ResourceHeader ResourceHeader = 2;
    types.CertAuthorityV2 CertAuthority = 3;
  }
}
`
	entries := []watchEntry{
		{Kind: "Cookie", Lower: "cookie", ProtoType: "teleport.cookie.v1.Cookie", FieldNum: 89},
		{Kind: "Webhook", Lower: "webhook", ProtoType: "teleport.webhook.v1.Webhook", FieldNum: 90},
	}
	result, err := insertWatchOneOfEntries(content, entries)
	require.NoError(t, err)

	require.Contains(t, result, "teleport.cookie.v1.Cookie Cookie = 89;")
	require.Contains(t, result, "teleport.webhook.v1.Webhook Webhook = 90;")
	require.Contains(t, result, "// Cookie is a resource for cookie management.")
	require.Contains(t, result, "// Webhook is a resource for webhook management.")

	// Existing entries are preserved.
	require.Contains(t, result, "types.ResourceHeader ResourceHeader = 2;")
	require.Contains(t, result, "types.CertAuthorityV2 CertAuthority = 3;")
}

func TestInsertWatchOneOfEntriesMissingOneof(t *testing.T) {
	content := `message Event {
  Operation Type = 1;
}
`
	entries := []watchEntry{
		{Kind: "Cookie", ProtoType: "teleport.cookie.v1.Cookie", FieldNum: 89},
	}
	_, err := insertWatchOneOfEntries(content, entries)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not find 'oneof Resource {'")
}
