package athena

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/secreports/query"
	"github.com/gravitational/teleport/lib/events"
	libathena "github.com/gravitational/teleport/lib/events/athena"
)

func TestAthenaRunQuery(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	ac := libathena.SetupAthenaContext(t, ctx, libathena.AthenaContextConfig{})

	auditLogger := &libathena.EventuallyConsistentAuditLogger{
		Inner:      ac.GetLog(),
		QueryDelay: time.Second * 15,
	}
	defer auditLogger.Close()

	awsConfig, err := config.LoadDefaultConfig(ctx)
	require.NoError(t, err)

	at := &Athena{
		client: athena.NewFromConfig(awsConfig),
		cfg: &Config{
			Database:         ac.Database,
			Table:            ac.TableName,
			QueryResults:     ac.S3ResultsLocation,
			Clock:            clockwork.NewRealClock(),
			QueryMaxDuration: time.Second * 10,
		},
	}

	t.Run("emit and query audit event", func(t *testing.T) {
		eventToEmit := &apievents.CertificateCreate{
			Metadata: apievents.Metadata{
				Type:        events.CertificateCreateEvent,
				Code:        events.CertificateCreateCode,
				ClusterName: "root",
			},
			CertificateType: events.CertificateTypeUser,
			Identity: &apievents.Identity{
				User:             "alice",
				Logins:           []string{"root"},
				KubernetesGroups: []string{"system:masters"},
			},
		}
		err := auditLogger.EmitAuditEvent(ctx, eventToEmit)
		require.NoError(t, err)

		ee, _, err := auditLogger.SearchEvents(ctx, events.SearchEventsRequest{
			From:       time.Now().Add(-1 * time.Hour),
			To:         time.Now().Add(time.Hour),
			EventTypes: []string{events.CertificateCreateEvent},
		})
		require.NoError(t, err)
		require.NotEmpty(t, ee)

		runResp, err := at.RunQuery(ctx, "select cluster_name as cluster_name, identity_user as user from cert_create limit 1;", 1)
		require.NoError(t, err)
		require.NotEmpty(t, runResp.ResultID)
		queryResultResp, err := at.GetQueryResult(ctx, runResp.ResultID, "", 0)
		require.NoError(t, err)

		want := []*query.Row{
			{Data: []string{"cluster_name", "user"}},
			{Data: []string{"root", "alice"}},
		}
		require.Equal(t, want, queryResultResp.Rows)
	})

	t.Run("test invalid query", func(t *testing.T) {
		_, err := at.RunQuery(ctx, "select * from invalidTable", 1)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("test pagination", func(t *testing.T) {
		queryText := `
, sequence AS (
	SELECT sequence_number FROM UNNEST(sequence(1, 2)) AS t(sequence_number)) SELECT 1 AS val FROM sequence
`
		runResp, err := at.RunQuery(ctx, queryText, 1)
		require.NoError(t, err)

		queryResultResp, err := at.GetQueryResult(ctx, runResp.ResultID, "", 1)
		require.NoError(t, err)

		want := []*query.Row{
			{Data: []string{"val"}},
		}
		require.Equal(t, want, queryResultResp.Rows)

		require.NotEmpty(t, queryResultResp.NextToken)
		queryResultResp, err = at.GetQueryResult(ctx, runResp.ResultID, queryResultResp.NextToken, 1)
		require.NoError(t, err)

		want = []*query.Row{
			{Data: []string{"1"}},
		}
		require.Equal(t, want, queryResultResp.Rows)
		require.NotEmpty(t, queryResultResp.NextToken)
	})
}
