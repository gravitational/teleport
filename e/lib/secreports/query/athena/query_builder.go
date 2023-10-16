// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package athena

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/gen/go/eventschema"
)

const (
	// viewQueryFmt is the view query format that filters the data based on the event_date.
	viewQueryFmt = "%s AND event_date BETWEEN current_date - interval '%d' day AND current_date"
	// viewSelectFmt is the view select format.
	viewSelectFmt = "%s FROM %s WHERE event_type='%s'"
)

// viewSchemaGetter is the view schema getter.
type viewSchemaGetter interface {
	// GetViewsDetails returns the view details.
	GetViewsDetails() ([]*eventschema.TableSchemaDetails, error)
}

// queryBuilder is the Athena query builder.
// The query is built dynamically based on the event schema.
type queryBuilder struct {
	views        viewSchemaGetter
	table        string
	daysInterval int
	sb           strings.Builder
}

// buildUserQuery builds the Athena query.
func (q *queryBuilder) buildUserQuery(query string) (string, error) {
	if err := q.buildDynamicVirtualViews(q.table, q.daysInterval); err != nil {
		return "", trace.Wrap(err)
	}
	// Since a query can be provided by the user we don't take any extract measure to forbid SQL Injection.
	// Though we guarantee that the query is executed in the context of the virtual view
	// and Athena client IAM permission allow only read access to athena audit event table.
	q.sb.WriteString(fmt.Sprintf("\n%s", query))
	return q.sb.String(), nil
}

// buildDynamicVirtualViews builds the dynamic virtual views.
// In order to reduce query complexity, additional virtual views are created.
// The virtual views are created based on the event schema.
func (q *queryBuilder) buildDynamicVirtualViews(table string, daysInterval int) error {
	// Days interval is used to filter the data based on the event_date.
	// where days 0 actually means today.
	// To get the data for the last 7 days, daysInterval should set to 6.
	daysInterval = daysInterval - 1

	eventsSchema, err := q.views.GetViewsDetails()
	if err != nil {
		return trace.Wrap(err)
	}

	for i, v := range eventsSchema {
		query := q.getViewSelect(v.CreateView(), v.Name, table)
		if err != nil {
			return trace.Wrap(err)
		}
		viewQuery := fmt.Sprintf(viewQueryFmt, query, daysInterval)
		if i == 0 {
			// The first view is created using 'WITH view_name AS (SELECT)' statement.
			q.sb.WriteString(fmt.Sprintf("WITH %s AS (\n %s\n)", v.SQLViewName, viewQuery))
		} else {
			// The rest of the views are appended using ', view_name AS (SELECT) statement'.
			q.sb.WriteString(fmt.Sprintf(",\n %s AS (\n %s)", v.SQLViewName, viewQuery))
		}
	}
	q.sb.WriteString("\n")
	return nil
}

// getViewSelect returns the view select statement.
func (q *queryBuilder) getViewSelect(viewViewQuery, eventName, table string) string {
	return fmt.Sprintf(viewSelectFmt, viewViewQuery, table, eventName)
}
