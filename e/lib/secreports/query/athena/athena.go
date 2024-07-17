package athena

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenaTypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"

	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/e/lib/secreports/metrics"
	"github.com/gravitational/teleport/e/lib/secreports/query"
	"github.com/gravitational/teleport/gen/go/eventschema"
)

const (
	// athenaSchema  is the Athena URI schema.
	athenaSchema = "athena"
)

const (
	// athenaUserErrorCategory is the Athena error category for user errors.
	// https://docs.aws.amazon.com/athena/latest/ug/error-reference.html
	athenaUserErrorCategory = 2
)

// Config is the Athena configuration use communicate with AWS API.
type Config struct {
	// Database is Athena Database name
	Database string
	// Table is the Athena Table.
	Table string
	// QueryResults is the S3 URI where the audit queries will be stored.
	QueryResults string
	// RoleARN is role arn for running Athena Audit Query.
	RoleARN string
	// Workgroup is the Athena Workgroup.
	Workgroup string
	// Clock is the clock used for testing.
	Clock clockwork.Clock
	// QueryMaxDuration is the maximum duration for a query to run.
	QueryMaxDuration time.Duration
	// QueryPullResultInterval is the interval for pulling query results.
	QueryPullResultInterval time.Duration
	// ReportResults is the S3 URI where the security reports will be stored.
	ReportResults string
	// AWSConfig is the AWS configuration.
	AWSConfig aws.Config
	// Log logs messages.
	Log *slog.Logger
}

// CheckAndSetDefaults checks and sets defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.Log == nil {
		c.Log = slog.Default()
	}
	if c.Database == "" {
		return trace.BadParameter("missing Database")
	}
	if c.Table == "" {
		return trace.BadParameter("missing Table")
	}
	if c.QueryResults == "" {
		return trace.BadParameter("missing QueryResult")
	}
	if c.RoleARN == "" {
		return trace.BadParameter("missing RoleARN")
	}
	if c.QueryMaxDuration == 0 {
		c.QueryMaxDuration = time.Minute * 30
	}
	if c.QueryPullResultInterval == 0 {
		c.QueryPullResultInterval = time.Second * 2
	}
	return nil
}

// GetAthenaURI returns the uri that contains Athena schema.
// If Athena schema URI was not found the false is value is
// returned as indication that Athena event store was not configured.
func GetAthenaURI(uris []string) (string, bool) {
	for _, v := range uris {
		if strings.HasPrefix(v, athenaSchema) {
			return v, true
		}
	}
	return "", false
}

const (
	// securityReportResultS3 is a S3 URI where the audit queries and security reports will be stored.
	securityReportResultS3 = "queryResultsS3"

	// securityReportRoleArn is a role ARN for running Athena Audit Query.
	// Security Reports needs to use different permissions than Audit Logs to prevent
	// user from modifying the audit logs where securityReportRoleArn allows only for read access.
	securityReportRoleArn = "securityReportRoleARN"

	// securityReportWorkgroup is a workgroup for running Athena Audit Query.
	securityReportWorkgroup = "securityReportWorkgroup"

	// reportResultS3 is a S3 URI where the security reports result will be stored.
	reportResultS3 = "accessMonitoringResultsS3"
)

// ConfigFromURI extract athena configuration from backend URL
func ConfigFromURI(uri string) (*Config, error) {
	url, err := apiutils.ParseSessionsURI(uri)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if url.Scheme != athenaSchema {
		return nil, trace.BadParameter("invalid URI schema")
	}
	parts := strings.Split(url.Host, ".")
	if len(parts) != 2 {
		return nil, trace.BadParameter("failed to find Athena URI")
	}
	return &Config{
		Database:      parts[0],
		Table:         parts[1],
		QueryResults:  url.Query().Get(securityReportResultS3),
		ReportResults: url.Query().Get(reportResultS3),
		RoleARN:       url.Query().Get(securityReportRoleArn),
		Workgroup:     url.Query().Get(securityReportWorkgroup),
	}, nil
}

// Athena struct is a helper struct used to aggregate athena API operations.
type Athena struct {
	client *athena.Client
	cfg    *Config
}

// NewAthena creates a new instance of Athena.
func NewAthena(athenaConf *Config, awsConfig aws.Config) (*Athena, error) {
	if err := athenaConf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Athena{
		client: athena.NewFromConfig(awsConfig),
		cfg:    athenaConf,
	}, nil
}

// RunQuery runs the Athena query.
func (a *Athena) RunQuery(ctx context.Context, inputQuery string, days int) (*query.RunQueryResponse, error) {
	qb := queryBuilder{
		views:        &genEventSchema{},
		table:        a.cfg.Table,
		daysInterval: days,
	}
	sqlQuery, err := qb.buildUserQuery(inputQuery)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := &athena.StartQueryExecutionInput{
		QueryString: aws.String(sqlQuery),
		ResultConfiguration: &athenaTypes.ResultConfiguration{
			OutputLocation: aws.String(a.cfg.QueryResults),
		},
		QueryExecutionContext: &athenaTypes.QueryExecutionContext{
			Database: aws.String(a.cfg.Database),
		},
	}

	if a.cfg.Workgroup != "" {
		input.WorkGroup = aws.String(a.cfg.Workgroup)
	}

	start := a.cfg.Clock.Now()
	status := "failed"
	defer func() {
		metrics.QueryExecutionTimeHist.With(
			prometheus.Labels{
				metrics.DaysTag:   fmt.Sprintf("%d", days),
				metrics.StatusTag: status,
			},
		).Observe(a.cfg.Clock.Since(start).Seconds())
	}()

	result, err := a.client.StartQueryExecution(ctx, input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	output, err := a.waitForSuccess(ctx, aws.ToString(result.QueryExecutionId))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if output.QueryExecution == nil {
		return nil, trace.BadParameter("queryExecution is nil")
	}
	status = "success"
	resp := query.RunQueryResponse{
		ResultID: aws.ToString(output.QueryExecution.QueryExecutionId),
	}
	if stat := output.QueryExecution.Statistics; stat != nil {
		resp.DataScannedInBytes = aws.ToInt64(stat.DataScannedInBytes)
		resp.TotalExecutionTimeInMillis = aws.ToInt64(stat.TotalExecutionTimeInMillis)
	}

	metrics.QueryScannedBytes.With(
		prometheus.Labels{metrics.DaysTag: fmt.Sprintf("%d", days)},
	).Add(float64(resp.DataScannedInBytes))

	return &resp, nil
}

// GetQueryResult returns query result from executing ID.
func (a *Athena) GetQueryResult(ctx context.Context, queryID, nextToken string, maxResults int32) (*query.GetQueryResultResponse, error) {
	result, err := a.client.GetQueryResults(ctx, &athena.GetQueryResultsInput{
		QueryExecutionId: aws.String(queryID),
		NextToken:        toAWSStringPtr(nextToken),
		MaxResults:       toAWSInt32Ptr(maxResults),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp := &query.GetQueryResultResponse{
		NextToken: aws.ToString(result.NextToken),
		QueryID:   queryID,
	}

	if rs := result.ResultSet; rs != nil {
		if rs.ResultSetMetadata != nil {
			resp.Columns = convertColumnInfo(rs.ResultSetMetadata.ColumnInfo)
		}
		resp.Rows = convertRows(rs.Rows)
	}
	return resp, nil
}

func convertRows(rows []athenaTypes.Row) []*query.Row {
	out := make([]*query.Row, 0, len(rows))
	for _, v := range rows {
		data := make([]string, 0, len(v.Data))
		for _, k := range v.Data {
			data = append(data, aws.ToString(k.VarCharValue))
		}
		out = append(out, &query.Row{Data: data})
	}
	return out
}

func convertColumnInfo(info []athenaTypes.ColumnInfo) []*query.ColumnInfo {
	out := make([]*query.ColumnInfo, 0, len(info))
	for _, v := range info {
		out = append(out, &query.ColumnInfo{
			Name: aws.ToString(v.Name),
			Type: aws.ToString(v.Type),
		})
	}
	return out
}

func (a *Athena) waitForSuccess(ctx context.Context, queryID string) (*athena.GetQueryExecutionOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, a.cfg.QueryMaxDuration)
	defer cancel()

	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		case <-a.cfg.Clock.After(a.cfg.QueryPullResultInterval):
		}

		resp, err := a.client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{QueryExecutionId: aws.String(queryID)})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if resp.QueryExecution.Status == nil {
			return nil, trace.Errorf("queryExecution status is nil")
		}
		queryStatus := resp.QueryExecution.Status
		switch queryStatus.State {
		case athenaTypes.QueryExecutionStateSucceeded:
			return resp, nil
		case athenaTypes.QueryExecutionStateCancelled, athenaTypes.QueryExecutionStateFailed:
			if queryStatus.AthenaError == nil || queryStatus.AthenaError.ErrorMessage == nil {
				return nil, trace.Errorf("athena query failed. Invalid response")
			}

			a.cfg.Log.WarnContext(ctx, "error running Athena query", "error", aws.ToString(queryStatus.AthenaError.ErrorMessage))
			switch aws.ToInt32(resp.QueryExecution.Status.AthenaError.ErrorCategory) {
			case athenaUserErrorCategory:
				// Return BadParameter error and format the error message to the user.
				return nil, trace.BadParameter("field to run user query: code %d", aws.ToInt32(queryStatus.AthenaError.ErrorType))
			default:
				// Return Internal error and hide the error information from the user.
				return nil, trace.Errorf("failed to run query")
			}
		case athenaTypes.QueryExecutionStateQueued, athenaTypes.QueryExecutionStateRunning:
			continue
		default:
			return nil, trace.Errorf("got unknown state: %s from queryID: %s", queryStatus.State, queryID)
		}
	}
}

func toAWSStringPtr(v string) *string {
	var vPtr *string
	if v != "" {
		vPtr = &v
	}
	return vPtr
}

func toAWSInt32Ptr(v int32) *int32 {
	var vPtr *int32
	if v != 0 {
		vPtr = &v
	}
	return vPtr
}

type genEventSchema struct{}

func (g *genEventSchema) GetViewsDetails() ([]*eventschema.TableSchemaDetails, error) {
	return eventschema.GetViewsDetails()
}
