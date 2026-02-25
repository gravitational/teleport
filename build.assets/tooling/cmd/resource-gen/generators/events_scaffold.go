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
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type eventMessage struct {
	Name        string // e.g. "CookieCreate"
	Lower       string // e.g. "cookie"
	Article     string // "a" or "an"
	OpPastTense string // e.g. "created"
}

type eventsProtoScaffoldData struct {
	Messages []eventMessage
}

var eventsProtoScaffoldTmpl = mustReadTemplate("events_proto_scaffold.proto.tmpl")

// GenerateEventsProtoScaffold renders a scaffold proto file for event messages.
// This file is created once per resource and never overwritten.
func GenerateEventsProtoScaffold(rs spec.ResourceSpec, module string) (string, error) {
	kind := rs.KindPascal
	lower := rs.Kind

	var msgs []eventMessage
	if rs.Audit.EmitOnCreate && rs.Operations.Create {
		msgs = append(msgs, eventMessage{Name: kind + "Create", Lower: lower, Article: indefiniteArticle(kind), OpPastTense: "created"})
	}
	if rs.Audit.EmitOnUpdate && (rs.Operations.Update || rs.Operations.Upsert) {
		msgs = append(msgs, eventMessage{Name: kind + "Update", Lower: lower, Article: indefiniteArticle(kind), OpPastTense: "updated"})
	}
	if rs.Audit.EmitOnDelete && rs.Operations.Delete {
		msgs = append(msgs, eventMessage{Name: kind + "Delete", Lower: lower, Article: indefiniteArticle(kind), OpPastTense: "deleted"})
	}
	if rs.Audit.EmitOnGet && rs.Operations.Get {
		msgs = append(msgs, eventMessage{Name: kind + "Get", Lower: lower, Article: indefiniteArticle(kind), OpPastTense: "read"})
	}

	if len(msgs) == 0 {
		return "", nil
	}

	data := eventsProtoScaffoldData{Messages: msgs}
	out, err := render("eventsProtoScaffold", eventsProtoScaffoldTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
