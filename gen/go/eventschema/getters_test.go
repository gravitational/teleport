package eventschema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTableSchema(t *testing.T) {
	schema, err := GetEventSchemaFromType("session.start")
	require.NoError(t, err)
	tableSchema, err := schema.TableSchema()
	require.NoError(t, err)
	require.NotEmpty(t, tableSchema)
}

func TestTableList(t *testing.T) {
	list, err := TableList()
	require.NoError(t, err)
	require.NotEmpty(t, list)
}

func TestViewSchema(t *testing.T) {
	schema, err := GetEventSchemaFromType("session.start")
	require.NoError(t, err)
	viewSchema, err := schema.ViewSchema()
	require.NoError(t, err)
	require.NotEmpty(t, viewSchema)
}
