/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package dynamo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/applicationautoscaling"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/applicationautoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

func marshalMapJSON(in map[string]types.AttributeValue) ([]byte, error) {
	out := make(map[string]any, len(in))
	for key, value := range in {
		switch value := value.(type) {
		case *types.AttributeValueMemberS:
			out[key] = map[string]string{"S": value.Value}
		case *types.AttributeValueMemberB:
			out[key] = map[string]string{"B": base64.StdEncoding.EncodeToString(value.Value)}
		default:
			return nil, fmt.Errorf("unsupported attribute value type %T", value)
		}
	}
	return json.Marshal(out)
}

func unmarshalMapJSON(data []byte) (map[string]types.AttributeValue, error) {
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	out := make(map[string]types.AttributeValue, len(raw))
	for key, value := range raw {
		if rawValue, ok := value["S"]; ok {
			var s string
			if err := json.Unmarshal(rawValue, &s); err != nil {
				return nil, err
			}
			out[key] = &types.AttributeValueMemberS{Value: s}
			continue
		}
		if rawValue, ok := value["B"]; ok {
			var s string
			if err := json.Unmarshal(rawValue, &s); err != nil {
				return nil, err
			}
			b, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, err
			}
			out[key] = &types.AttributeValueMemberB{Value: b}
			continue
		}
		return nil, fmt.Errorf("unsupported attribute value JSON for %q", key)
	}
	return out, nil
}

func newTestDynamoBackend(server *httptest.Server) *Backend {
	return &Backend{
		svc: dynamodb.New(dynamodb.Options{
			Region:       "us-west-2",
			Credentials:  credentials.NewStaticCredentialsProvider("access-key", "secret-key", ""),
			BaseEndpoint: aws.String(server.URL),
			HTTPClient:   server.Client(),
		}),
		clock:  clockwork.NewRealClock(),
		logger: slog.Default(),
		Config: Config{
			TableName:   "table",
			RetryPeriod: 10 * time.Second,
		},
	}
}

type listenerAddrOverride struct {
	*bufconn.Listener
}

func (l listenerAddrOverride) Addr() net.Addr { return &utils.NetAddr{Addr: "example.com:80"} }

func TestDeleteRangeRetriesUnprocessedItems(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		keys := []backend.Key{
			backend.NewKey("testing", "fish"),
			backend.NewKey("testing", "dumpling"),
			backend.NewKey("testing", "color", "red"),
		}

		var batchWriteRequests [][]string
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Header.Get("X-Amz-Target") {
			case "DynamoDB_20120810.Query":
				items := make([]json.RawMessage, 0, len(keys))
				for _, key := range keys {
					item := map[string]types.AttributeValue{
						hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
						fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(key)},
						"Value":     &types.AttributeValueMemberB{Value: []byte("value")},
					}

					rawItem, err := marshalMapJSON(item)
					require.NoError(t, err)
					items = append(items, rawItem)
				}

				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				require.NoError(t, json.NewEncoder(w).Encode(map[string][]json.RawMessage{"Items": items}))
			case "DynamoDB_20120810.BatchWriteItem":
				var body map[string]map[string][]json.RawMessage
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

				var requestedKeys []string
				for _, requestedItems := range body["RequestItems"] {
					for _, request := range requestedItems {
						var raw map[string]json.RawMessage
						require.NoError(t, json.Unmarshal(request, &raw))

						if deleteRequest, ok := raw["DeleteRequest"]; ok {
							var fields map[string]json.RawMessage
							require.NoError(t, json.Unmarshal(deleteRequest, &fields))
							key, err := unmarshalMapJSON(fields["Key"])
							require.NoError(t, err)

							fullPath, ok := key[fullPathKey].(*types.AttributeValueMemberS)
							require.True(t, ok)
							requestedKeys = append(requestedKeys, fullPath.Value)
						}
					}
				}

				batchWriteRequests = append(batchWriteRequests, requestedKeys)

				if len(batchWriteRequests) == 3 {
					return
				}

				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				switch len(batchWriteRequests) {
				case 1:
					rawKey1, err := marshalMapJSON(map[string]types.AttributeValue{
						hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
						fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(keys[1])},
					})
					require.NoError(t, err)

					rawRequest1, err := json.Marshal(map[string]any{
						"DeleteRequest": map[string]json.RawMessage{
							"Key": rawKey1,
						},
					})
					require.NoError(t, err)

					rawKey2, err := marshalMapJSON(map[string]types.AttributeValue{
						hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
						fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(keys[2])},
					})
					require.NoError(t, err)

					rawRequest2, err := json.Marshal(map[string]any{
						"DeleteRequest": map[string]json.RawMessage{
							"Key": rawKey2,
						},
					})
					require.NoError(t, err)

					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"UnprocessedItems": map[string][]json.RawMessage{"table": {rawRequest1, rawRequest2}},
					}))
				case 2:
					rawKey, err := marshalMapJSON(map[string]types.AttributeValue{
						hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
						fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(keys[2])},
					})
					require.NoError(t, err)

					rawRequest, err := json.Marshal(map[string]any{
						"DeleteRequest": map[string]json.RawMessage{
							"Key": rawKey,
						},
					})
					require.NoError(t, err)

					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"UnprocessedItems": map[string][]json.RawMessage{"table": {rawRequest}},
					}))
				}
			default:
				http.Error(w, "unexpected dynamodb operation", http.StatusBadRequest)
			}
		})

		listener := bufconn.Listen(1024)
		server := &httptest.Server{
			Listener: listenerAddrOverride{Listener: listener},
			Config:   &http.Server{Handler: handler},
		}
		server.StartTLS()
		t.Cleanup(server.Close)

		client := server.Client()
		transport := client.Transport.(*http.Transport).Clone()
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}
		client.Transport = transport

		bk := newTestDynamoBackend(server)

		err := bk.DeleteRange(t.Context(), backend.NewKey("foo"), backend.RangeEnd(backend.NewKey("zest")))
		require.NoError(t, err)

		expectedBatchWriteRequests := [][]string{
			{prependPrefix(keys[0]), prependPrefix(keys[1]), prependPrefix(keys[2])},
			{prependPrefix(keys[1]), prependPrefix(keys[2])},
			{prependPrefix(keys[2])},
		}
		require.Len(t, batchWriteRequests, len(expectedBatchWriteRequests))
		for i := range expectedBatchWriteRequests {
			require.ElementsMatch(t, expectedBatchWriteRequests[i], batchWriteRequests[i])
		}
	})
}

func TestDeleteRangeReturnsErrorWhenUnprocessedItemsDoNotDecrease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		key := backend.NewKey("testing", "test")
		var batchWriteAttempts int
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Header.Get("X-Amz-Target") {
			case "DynamoDB_20120810.Query":
				item := map[string]types.AttributeValue{
					hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
					fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(key)},
					"Value":     &types.AttributeValueMemberB{Value: []byte("value")},
				}

				rawItem, err := marshalMapJSON(item)
				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				require.NoError(t, json.NewEncoder(w).Encode(map[string][]json.RawMessage{"Items": {rawItem}}))
			case "DynamoDB_20120810.BatchWriteItem":
				batchWriteAttempts++
				rawKey, err := marshalMapJSON(map[string]types.AttributeValue{
					hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
					fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(key)},
				})
				require.NoError(t, err)

				rawRequest, err := json.Marshal(map[string]any{
					"DeleteRequest": map[string]json.RawMessage{
						"Key": rawKey,
					},
				})
				require.NoError(t, err)
				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"UnprocessedItems": map[string][]json.RawMessage{"table": {rawRequest}},
				}))
			default:
				http.Error(w, "unexpected dynamodb operation", http.StatusBadRequest)
			}
		})

		listener := bufconn.Listen(1024)
		server := &httptest.Server{
			Listener: listenerAddrOverride{Listener: listener},
			Config:   &http.Server{Handler: handler},
		}
		server.StartTLS()
		t.Cleanup(server.Close)

		client := server.Client()
		transport := client.Transport.(*http.Transport).Clone()
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}
		client.Transport = transport

		bk := newTestDynamoBackend(server)

		err := bk.DeleteRange(t.Context(), backend.NewKey("test"), backend.RangeEnd(backend.NewKey("zest")))
		require.Error(t, err)
		require.True(t, trace.IsLimitExceeded(err))
		require.Equal(t, 1, batchWriteAttempts)
	})
}

func TestDeleteRangeReturnsErrorWhenUnprocessedItemsIncrease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		key := backend.NewKey("testing", "test")
		extraKey := backend.NewKey("testing", "extra")
		var batchWriteAttempts int
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Header.Get("X-Amz-Target") {
			case "DynamoDB_20120810.Query":
				item := map[string]types.AttributeValue{
					hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
					fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(key)},
					"Value":     &types.AttributeValueMemberB{Value: []byte("value")},
				}

				rawItem, err := marshalMapJSON(item)
				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				require.NoError(t, json.NewEncoder(w).Encode(map[string][]json.RawMessage{"Items": {rawItem}}))
			case "DynamoDB_20120810.BatchWriteItem":
				batchWriteAttempts++

				rawKey, err := marshalMapJSON(map[string]types.AttributeValue{
					hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
					fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(key)},
				})
				require.NoError(t, err)

				rawRequest, err := json.Marshal(map[string]any{
					"DeleteRequest": map[string]json.RawMessage{
						"Key": rawKey,
					},
				})
				require.NoError(t, err)

				rawExtraKey, err := marshalMapJSON(map[string]types.AttributeValue{
					hashKeyKey:  &types.AttributeValueMemberS{Value: hashKey},
					fullPathKey: &types.AttributeValueMemberS{Value: prependPrefix(extraKey)},
				})
				require.NoError(t, err)

				rawExtraRequest, err := json.Marshal(map[string]any{
					"DeleteRequest": map[string]json.RawMessage{
						"Key": rawExtraKey,
					},
				})
				require.NoError(t, err)

				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"UnprocessedItems": map[string][]json.RawMessage{"table": {rawRequest, rawExtraRequest}},
				}))
			default:
				http.Error(w, "unexpected dynamodb operation", http.StatusBadRequest)
			}
		})

		listener := bufconn.Listen(1024)
		server := &httptest.Server{
			Listener: listenerAddrOverride{Listener: listener},
			Config:   &http.Server{Handler: handler},
		}
		server.StartTLS()
		t.Cleanup(server.Close)

		client := server.Client()
		transport := client.Transport.(*http.Transport).Clone()
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}
		client.Transport = transport

		bk := newTestDynamoBackend(server)

		err := bk.DeleteRange(t.Context(), backend.NewKey("test"), backend.RangeEnd(backend.NewKey("zest")))
		require.Error(t, err)
		require.True(t, trace.IsLimitExceeded(err))
		require.Equal(t, 1, batchWriteAttempts)
	})
}

func ensureTestsEnabled(t *testing.T) {
	t.Helper()
	const varName = "TELEPORT_DYNAMODB_TEST"
	if os.Getenv(varName) == "" {
		t.Skipf("DynamoDB tests are disabled. Enable by defining the %v environment variable", varName)
	}
}

func dynamoDBTestTable() string {
	if t := os.Getenv("TELEPORT_DYNAMODB_TEST_TABLE"); t != "" {
		return t
	}

	return "teleport.dynamo.test"
}

func TestDynamoDB(t *testing.T) {
	ensureTestsEnabled(t)

	dynamoCfg := map[string]any{
		"table_name":         dynamoDBTestTable(),
		"poll_stream_period": 300 * time.Millisecond,
	}

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		testCfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if testCfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		// This would seem to be a bad thing for dynamo to omit
		if testCfg.ConcurrentBackend != nil {
			return nil, nil, test.ErrConcurrentAccessNotSupported
		}

		uut, err := New(context.Background(), dynamoCfg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return uut, test.BlockingFakeClock{Clock: clockwork.NewRealClock()}, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

type dynamoDBAPIMock struct {
	dynamoClient
	expectedProvisionedthroughput *types.ProvisionedThroughput
	expectedTableName             string
	expectedBillingMode           types.BillingMode
}

func (d *dynamoDBAPIMock) CreateTable(ctx context.Context, input *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	if d.expectedTableName != aws.ToString(input.TableName) {
		return nil, trace.BadParameter("table names do not match")
	}

	if d.expectedBillingMode != input.BillingMode {
		return nil, trace.BadParameter("billing mode does not match")
	}

	if d.expectedProvisionedthroughput != nil {
		if input.BillingMode == types.BillingModePayPerRequest {
			return nil, trace.BadParameter("pthroughput should be nil if on demand is true")
		}

		if aws.ToInt64(d.expectedProvisionedthroughput.ReadCapacityUnits) != aws.ToInt64(input.ProvisionedThroughput.ReadCapacityUnits) ||
			aws.ToInt64(d.expectedProvisionedthroughput.WriteCapacityUnits) != aws.ToInt64(input.ProvisionedThroughput.WriteCapacityUnits) {

			return nil, trace.BadParameter("pthroughput values were not equal")
		}
	}

	return nil, nil
}

func (d *dynamoDBAPIMock) DescribeTable(ctx context.Context, input *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if d.expectedTableName != aws.ToString(input.TableName) {
		return nil, trace.BadParameter("table names do not match")
	}
	return &dynamodb.DescribeTableOutput{
		Table: &types.TableDescription{
			TableName:   input.TableName,
			TableStatus: types.TableStatusActive,
		},
		ResultMetadata: middleware.Metadata{},
	}, nil
}

func TestCreateTable(t *testing.T) {
	const tableName = "table"

	errIsNil := func(err error) bool { return err == nil }

	for _, tc := range []struct {
		errorIsFn                     func(error) bool
		expectedProvisionedThroughput *types.ProvisionedThroughput
		name                          string
		expectedBillingMode           types.BillingMode
		billingMode                   billingMode
		readCapacityUnits             int
		writeCapacityUnits            int
	}{
		{
			name:                "table creation succeeds",
			errorIsFn:           errIsNil,
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:                "read/write capacity units are ignored if on demand is on",
			readCapacityUnits:   10,
			writeCapacityUnits:  10,
			errorIsFn:           errIsNil,
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:               "bad parameter when provisioned throughput is set",
			readCapacityUnits:  10,
			writeCapacityUnits: 10,
			errorIsFn:          trace.IsBadParameter,
			expectedProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:               "bad parameter when the incorrect billing mode is set",
			readCapacityUnits:  10,
			writeCapacityUnits: 10,
			errorIsFn:          trace.IsBadParameter,
			expectedProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			billingMode:         billingModePayPerRequest,
			expectedBillingMode: types.BillingModePayPerRequest,
		},
		{
			name:               "create table succeeds",
			readCapacityUnits:  10,
			writeCapacityUnits: 10,
			errorIsFn:          errIsNil,
			expectedProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			billingMode:         billingModeProvisioned,
			expectedBillingMode: types.BillingModeProvisioned,
		},
	} {

		ctx := context.Background()
		t.Run(tc.name, func(t *testing.T) {
			mock := dynamoDBAPIMock{
				expectedBillingMode:           tc.expectedBillingMode,
				expectedTableName:             tableName,
				expectedProvisionedthroughput: tc.expectedProvisionedThroughput,
			}
			b := &Backend{
				logger: slog.With(teleport.ComponentKey, BackendName),
				Config: Config{
					BillingMode:        tc.billingMode,
					ReadCapacityUnits:  int64(tc.readCapacityUnits),
					WriteCapacityUnits: int64(tc.writeCapacityUnits),
				},

				svc: &mock,
			}

			err := b.createTable(ctx, aws.String(tableName), "_")
			require.True(t, tc.errorIsFn(err), err)
		})
	}
}

// TestContinuousBackups verifies that the continuous backup state is set upon
// startup of DynamoDB.
func TestContinuousBackups(t *testing.T) {
	ensureTestsEnabled(t)

	b, err := New(t.Context(), map[string]any{
		"table_name":         uuid.NewString() + "-test",
		"continuous_backups": true,
	})
	require.NoError(t, err)

	// Remove table after tests are done.
	t.Cleanup(func() {
		retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
			Driver: retryutils.NewExponentialDriver(500 * time.Millisecond),
			First:  500 * time.Millisecond,
			Max:    20 * time.Second,
			Jitter: retryutils.HalfJitter,
		})
		require.NoError(t, err)

		for {
			err := deleteTable(context.Background(), b.svc, b.Config.TableName)
			if err == nil {
				return
			}
			inUse := &types.ResourceInUseException{}
			if errors.As(err, &inUse) {
				<-retry.After()
			} else {
				assert.FailNow(t, "error deleting table", err)
			}
		}
	})

	// Check status of continuous backups.
	ok, err := getContinuousBackups(context.Background(), b.svc, b.Config.TableName)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestAutoScaling verifies that auto scaling is enabled upon startup of DynamoDB.
func TestAutoScaling(t *testing.T) {
	ensureTestsEnabled(t)

	// Create new backend with auto scaling enabled.
	b, err := New(context.Background(), map[string]any{
		"table_name":         uuid.NewString() + "-test",
		"auto_scaling":       true,
		"read_min_capacity":  10,
		"read_max_capacity":  20,
		"read_target_value":  50.0,
		"write_min_capacity": 10,
		"write_max_capacity": 20,
		"write_target_value": 50.0,
		// Billing mode must be set to provisioned mode for the
		// auto-scaling option to be respected.
		"billing_mode": billingModeProvisioned,
	})
	require.NoError(t, err)

	// Remove table after tests are done.
	t.Cleanup(func() {
		require.NoError(t, deleteTable(context.Background(), b.svc, b.Config.TableName))
	})

	awsConfig, err := config.LoadDefaultConfig(context.Background())
	require.NoError(t, err)

	expected := &AutoScalingParams{
		ReadMinCapacity:  10,
		ReadMaxCapacity:  20,
		ReadTargetValue:  50.0,
		WriteMinCapacity: 10,
		WriteMaxCapacity: 20,
		WriteTargetValue: 50.0,
	}

	// Check auto scaling values match.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		resp, err := getAutoScaling(context.Background(), applicationautoscaling.NewFromConfig(awsConfig), b.Config.TableName)
		require.NoError(t, err)
		require.Equal(t, expected, resp)
	}, 10*time.Second, 500*time.Millisecond)
}

// getContinuousBackups gets the state of continuous backups.
func getContinuousBackups(ctx context.Context, svc dynamoClient, tableName string) (bool, error) {
	resp, err := svc.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return false, convertError(err)
	}

	switch resp.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus {
	case types.PointInTimeRecoveryStatusEnabled:
		return true, nil
	case types.PointInTimeRecoveryStatusDisabled:
		return false, nil
	default:
		return false, trace.BadParameter("dynamo returned unknown state for continuous backups: %v",
			resp.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus)
	}
}

type AutoScalingParams struct {
	// ReadMaxCapacity is the maximum provisioned read capacity.
	ReadMaxCapacity int32
	// ReadMinCapacity is the minimum provisioned read capacity.
	ReadMinCapacity int32
	// ReadTargetValue is the ratio of consumed read to provisioned capacity.
	ReadTargetValue float64
	// WriteMaxCapacity is the maximum provisioned write capacity.
	WriteMaxCapacity int32
	// WriteMinCapacity is the minimum provisioned write capacity.
	WriteMinCapacity int32
	// WriteTargetValue is the ratio of consumed write to provisioned capacity.
	WriteTargetValue float64
}

// getAutoScaling gets the state of auto scaling.
func getAutoScaling(ctx context.Context, svc *applicationautoscaling.Client, tableName string) (*AutoScalingParams, error) {
	var resp AutoScalingParams
	tableResourceID := "table/" + tableName

	// Get scaling targets.
	targetResponse, err := svc.DescribeScalableTargets(ctx, &applicationautoscaling.DescribeScalableTargetsInput{
		ServiceNamespace: autoscalingtypes.ServiceNamespaceDynamodb,
		ResourceIds:      []string{tableResourceID},
	})
	if err != nil {
		return nil, convertError(err)
	}
	for _, target := range targetResponse.ScalableTargets {
		switch target.ScalableDimension {
		case autoscalingtypes.ScalableDimensionDynamoDBTableReadCapacityUnits:
			resp.ReadMinCapacity = aws.ToInt32(target.MinCapacity)
			resp.ReadMaxCapacity = aws.ToInt32(target.MaxCapacity)
		case autoscalingtypes.ScalableDimensionDynamoDBTableWriteCapacityUnits:
			resp.WriteMinCapacity = aws.ToInt32(target.MinCapacity)
			resp.WriteMaxCapacity = aws.ToInt32(target.MaxCapacity)
		}
	}

	// Get scaling policies.
	policyResponse, err := svc.DescribeScalingPolicies(ctx, &applicationautoscaling.DescribeScalingPoliciesInput{
		ServiceNamespace: autoscalingtypes.ServiceNamespaceDynamodb,
		ResourceId:       aws.String(tableResourceID),
	})
	if err != nil {
		return nil, convertError(err)
	}
	for i := range policyResponse.ScalingPolicies {
		policy := policyResponse.ScalingPolicies[i]
		switch aws.ToString(policy.PolicyName) {
		case fmt.Sprintf("%v-%v", tableName, readScalingPolicySuffix):
			resp.ReadTargetValue = aws.ToFloat64(policy.TargetTrackingScalingPolicyConfiguration.TargetValue)
		case fmt.Sprintf("%v-%v", tableName, writeScalingPolicySuffix):
			resp.WriteTargetValue = aws.ToFloat64(policy.TargetTrackingScalingPolicyConfiguration.TargetValue)
		}
	}

	return &resp, nil
}

// deleteTable will remove a table.
func deleteTable(ctx context.Context, svc dynamoClient, tableName string) error {
	_, err := svc.DeleteTable(ctx, &dynamodb.DeleteTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return convertError(err)
	}

	waiter := dynamodb.NewTableNotExistsWaiter(svc)
	if err := waiter.Wait(ctx,
		&dynamodb.DescribeTableInput{
			TableName: aws.String(tableName),
		},
		10*time.Minute,
	); err != nil {
		return convertError(err)
	}
	return nil
}

const (
	readScalingPolicySuffix  = "read-target-tracking-scaling-policy"
	writeScalingPolicySuffix = "write-target-tracking-scaling-policy"
)

func TestKeyPrefix(t *testing.T) {
	t.Run("leading separator in key", func(t *testing.T) {
		prefixed := prependPrefix(backend.NewKey("test", "llama"))
		assert.Equal(t, "teleport/test/llama", prefixed)

		key := trimPrefix(prefixed)
		assert.Equal(t, "/test/llama", key.String())
	})

	t.Run("no leading separator in key", func(t *testing.T) {
		prefixed := prependPrefix(backend.KeyFromString(".locks/test/llama"))
		assert.Equal(t, "teleport.locks/test/llama", prefixed)

		key := trimPrefix(prefixed)
		assert.Equal(t, ".locks/test/llama", key.String())
	})
}
