package query

import pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"

// RunQueryResponse is the response for the RunQuery request.
type RunQueryResponse struct {
	// ResultID is the query result ID that can be used to get the query result.
	ResultID string
	// TotalExecutionTimeInMillis is the total execution time in milliseconds.
	TotalExecutionTimeInMillis int64
	// DataScannedInBytes is the data scanned in bytes.
	DataScannedInBytes int64
}

// ColumnInfo is the column information.
type ColumnInfo struct {
	// Name is the column name.
	Name string
	// Type is the column type.
	Type string
}

// Row is the row.
type Row struct {
	// Data is the row data.
	Data []string
}

// GetQueryResultResponse is the response for the GetQueryResult request.
type GetQueryResultResponse struct {
	// NextToken is the next token.
	NextToken string
	// QueryID is the query ID.
	QueryID string
	// Columns is the list of columns.
	Columns []*ColumnInfo
	// Rows is the list of rows.
	Rows []*Row
}

// HasMoreData returns true if there is more data.
func (r *GetQueryResultResponse) HasMoreData() bool {
	return r.NextToken != ""
}

// ToProto converts the response to the protobuf format.
func (r *GetQueryResultResponse) ToProto() *pb.QueryResultSet {
	var out pb.QueryResultSet
	for _, v := range r.Columns {
		out.SetColumnInfo(append(out.GetColumnInfo(), pb.QueryResultColumnInfo_builder{
			Name: v.Name,
			Type: v.Type,
		}.Build()))
	}
	for _, v := range r.Rows {
		out.SetRows(append(out.GetRows(), pb.QueryRowResult_builder{
			Data: v.Data,
		}.Build()))
	}
	return &out
}
