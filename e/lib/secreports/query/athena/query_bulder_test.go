package athena

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/gen/go/eventschema"
)

type viewsMock struct {
	views []*eventschema.TableSchemaDetails
}

func (v *viewsMock) GetViewsDetails() ([]*eventschema.TableSchemaDetails, error) {
	return v.views, nil
}

func TestBuildUserQuery(t *testing.T) {
	t.Parallel()
	viewsSchema := []*eventschema.TableSchemaDetails{
		{
			Name:        "user.cert",
			SQLViewName: "user_cert",
			Columns:     []*eventschema.ColumnSchemaDetails{{Path: []string{"identity", "login"}, Type: "string"}},
		},
		{
			Name:        "create.cert",
			SQLViewName: "create_cert",
			Columns:     []*eventschema.ColumnSchemaDetails{{Path: []string{"user"}, Type: "string"}},
		},
	}
	qb := queryBuilder{
		views: &viewsMock{
			views: viewsSchema,
		},
		table:        "my_table",
		daysInterval: 1,
	}

	want := `WITH user_cert AS (
 SELECT
 event_date, event_time
  , CAST(json_extract(event_data, '$["identity"]["login"]') AS string) as identity_login
 FROM my_table WHERE event_type='user.cert' AND event_date BETWEEN current_date - interval '0' day AND current_date
),
 create_cert AS (
 SELECT
 event_date, event_time
  , CAST(json_extract(event_data, '$["user"]') AS string) as user
 FROM my_table WHERE event_type='create.cert' AND event_date BETWEEN current_date - interval '0' day AND current_date)

SELECT * FROM user_cert`

	got, err := qb.buildUserQuery("SELECT * FROM user_cert")
	require.NoError(t, err)
	require.Equal(t, want, got)
}
