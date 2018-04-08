/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

package dynamo

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

// DynamoConfig structure represents DynamoDB confniguration as appears in `storage` section
// of Teleport YAML
type DynamoConfig struct {
	// Region is where DynamoDB Table will be used to store k/v
	Region string `json:"region,omitempty"`
	// AWS AccessKey used to authenticate DynamoDB queries (prefer IAM role instead of hardcoded value)
	AccessKey string `json:"access_key,omitempty"`
	// AWS SecretKey used to authenticate DynamoDB queries (prefer IAM role instead of hardcoded value)
	SecretKey string `json:"secret_key,omitempty"`
	// Tablename where to store K/V in DynamoDB
	Tablename string `json:"table_name,omitempty"`
	// ReadCapacityUnits is Dynamodb read capacity units
	ReadCapacityUnits int64 `json:"read_capacity_units"`
	// WriteCapacityUnits is Dynamodb write capacity units
	WriteCapacityUnits int64 `json:"write_capacity_units"`
}

// CheckAndSetDefaults is a helper returns an error if the supplied configuration
// is not enough to connect to DynamoDB
func (cfg *DynamoConfig) CheckAndSetDefaults() error {
	// table is not configured?
	if cfg.Tablename == "" {
		return trace.BadParameter("DynamoDB: table_name is not specified")
	}
	if cfg.ReadCapacityUnits == 0 {
		cfg.ReadCapacityUnits = DefaultReadCapacityUnits
	}
	if cfg.WriteCapacityUnits == 0 {
		cfg.WriteCapacityUnits = DefaultWriteCapacityUnits
	}
	return nil
}

// DynamoDBBackend struct
type DynamoDBBackend struct {
	*log.Entry
	DynamoConfig
	svc   *dynamodb.DynamoDB
	clock clockwork.Clock
}

type record struct {
	HashKey   string
	FullPath  string
	Value     []byte
	Timestamp int64
	TTL       time.Duration
	Expires   *int64 `json:"Expires,omitempty"`
	key       string
}

type keyLookup struct {
	HashKey  string
	FullPath string
}

const (
	// hashKey is actually the name of the partition. This backend
	// places all objects in the same DynamoDB partition
	hashKey = "teleport"

	// obsolete schema key. if a table contains "Key" column it means
	// such table needs to be migrated
	oldPathAttr = "Key"

	// BackendName is the name of this backend
	BackendName = "dynamodb"

	// ttlKey is a key used for TTL specification
	ttlKey = "Expires"

	// DefaultReadCapacityUnits specifies default value for read capacity units
	DefaultReadCapacityUnits = 10

	// DefaultWriteCapacityUnits specifies default value for write capacity units
	DefaultWriteCapacityUnits = 10
)

// GetName() is a part of backend API and it returns DynamoDB backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return BackendName
}

// New returns new instance of DynamoDB backend.
// It's an implementation of backend API's NewFunc
func New(params backend.Params) (backend.Backend, error) {
	l := log.WithFields(log.Fields{trace.Component: BackendName})
	l.Info("Initializing backend.")

	var cfg *DynamoConfig
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		log.Error(err)
		return nil, trace.BadParameter("DynamoDB configuration is invalid", err)
	}

	defer l.Debug("AWS session is created.")

	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	b := &DynamoDBBackend{
		Entry:        l,
		DynamoConfig: *cfg,
		clock:        clockwork.NewRealClock(),
	}
	// create an AWS session using default SDK behavior, i.e. it will interpret
	// the environment and ~/.aws directory just like an AWS CLI tool would:
	sess, err := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// override the default environment (region + credentials) with the values
	// from the YAML file:
	if cfg.Region != "" {
		sess.Config.Region = aws.String(cfg.Region)
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		creds := credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, "")
		sess.Config.Credentials = creds
	}

	// create DynamoDB service:
	b.svc = dynamodb.New(sess)

	// check if the table exists?
	ts, err := b.getTableStatus(b.Tablename)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch ts {
	case tableStatusOK:
		break
	case tableStatusMissing:
		err = b.createTable(b.Tablename, "FullPath")
	case tableStatusNeedsMigration:
		return nil, trace.BadParameter("unsupported schema")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.turnOnTimeToLive()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

type tableStatus int

const (
	tableStatusError = iota
	tableStatusMissing
	tableStatusNeedsMigration
	tableStatusOK
)

// Clock returns wall clock
func (b *DynamoDBBackend) Clock() clockwork.Clock {
	return b.clock
}

func (b *DynamoDBBackend) turnOnTimeToLive() error {
	status, err := b.svc.DescribeTimeToLive(&dynamodb.DescribeTimeToLiveInput{
		TableName: aws.String(b.Tablename),
	})
	if err != nil {
		return trace.Wrap(convertError(err))
	}
	switch aws.StringValue(status.TimeToLiveDescription.TimeToLiveStatus) {
	case dynamodb.TimeToLiveStatusEnabled, dynamodb.TimeToLiveStatusEnabling:
		return nil
	}
	_, err = b.svc.UpdateTimeToLive(&dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(b.Tablename),
		TimeToLiveSpecification: &dynamodb.TimeToLiveSpecification{
			AttributeName: aws.String(ttlKey),
			Enabled:       aws.Bool(true),
		},
	})
	return convertError(err)
}

// getTableStatus checks if a given table exists
func (b *DynamoDBBackend) getTableStatus(tableName string) (tableStatus, error) {
	td, err := b.svc.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	err = convertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return tableStatusMissing, nil
		}
		return tableStatusError, trace.Wrap(err)
	}
	for _, attr := range td.Table.AttributeDefinitions {
		if *attr.AttributeName == oldPathAttr {
			return tableStatusNeedsMigration, nil
		}
	}
	return tableStatusOK, nil
}

// createTable creates a DynamoDB table with a requested name and applies
// the back-end schema to it. The table must not exist.
//
// rangeKey is the name of the 'range key' the schema requires.
// currently is always set to "FullPath" (used to be something else, that's
// why it's a parameter for migration purposes)
func (b *DynamoDBBackend) createTable(tableName string, rangeKey string) error {
	pThroughput := dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(b.ReadCapacityUnits),
		WriteCapacityUnits: aws.Int64(b.WriteCapacityUnits),
	}
	def := []*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("HashKey"),
			AttributeType: aws.String("S"),
		},
		{
			AttributeName: aws.String(rangeKey),
			AttributeType: aws.String("S"),
		},
	}
	elems := []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("HashKey"),
			KeyType:       aws.String("HASH"),
		},
		{
			AttributeName: aws.String(rangeKey),
			KeyType:       aws.String("RANGE"),
		},
	}
	c := dynamodb.CreateTableInput{
		TableName:             aws.String(tableName),
		AttributeDefinitions:  def,
		KeySchema:             elems,
		ProvisionedThroughput: &pThroughput,
	}
	_, err := b.svc.CreateTable(&c)
	if err != nil {
		return trace.Wrap(err)
	}
	b.Infof("Waiting until table %q is created.", tableName)
	err = b.svc.WaitUntilTableExists(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		b.Infof("Table %q has been created.", tableName)
	}
	return trace.Wrap(err)
}

// deleteTable deletes DynamoDB table with a given name
func (b *DynamoDBBackend) deleteTable(tableName string, wait bool) error {
	tn := aws.String(tableName)
	_, err := b.svc.DeleteTable(&dynamodb.DeleteTableInput{TableName: tn})
	if err != nil {
		return trace.Wrap(err)
	}
	if wait {
		return trace.Wrap(
			b.svc.WaitUntilTableNotExists(&dynamodb.DescribeTableInput{TableName: tn}))
	}
	return nil
}

// Close the DynamoDB driver
func (b *DynamoDBBackend) Close() error {
	return nil
}

func (b *DynamoDBBackend) fullPath(bucket ...string) string {
	return strings.Join(append([]string{"teleport"}, bucket...), "/")
}

// getRecords retrieve all prefixed keys
func (b *DynamoDBBackend) getRecords(path string) ([]record, error) {
	var vals []record
	query := "HashKey = :hashKey AND begins_with (FullPath, :fullPath)"
	attrV := map[string]interface{}{
		":fullPath":  path,
		":hashKey":   hashKey,
		":timestamp": b.clock.Now().UTC().Unix(),
	}
	// filter out expired items, otherwise they might show up in the query
	// http://docs.aws.amazon.com/amazondynamodb/latest/developerguide/howitworks-ttl.html
	filter := fmt.Sprintf("attribute_not_exists(Expires) OR Expires >= :timestamp")
	av, err := dynamodbattribute.MarshalMap(attrV)
	input := dynamodb.QueryInput{
		KeyConditionExpression:    aws.String(query),
		TableName:                 &b.Tablename,
		ExpressionAttributeValues: av,
		FilterExpression:          aws.String(filter),
	}
	out, err := b.svc.Query(&input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: manage paginated result otherwise only up to 1M (max) of data will be returned.
	for _, item := range out.Items {
		var r record
		dynamodbattribute.UnmarshalMap(item, &r)

		if strings.Compare(path, r.FullPath[:len(path)]) == 0 && len(path) < len(r.FullPath) {
			if r.isExpired() {
				b.deleteKey(r.FullPath)
			} else {
				r.key = suffix(r.FullPath[len(path)+1:])
				vals = append(vals, r)
			}
		}
	}
	sort.Sort(records(vals))
	vals = removeDuplicates(vals)
	return vals, nil
}

// isExpired returns 'true' if the given object (record) has a TTL and
// it's due.
func (r *record) isExpired() bool {
	if r.TTL == 0 {
		return false
	}
	expiryDateUTC := time.Unix(r.Timestamp, 0).Add(r.TTL).UTC()
	nowUTC := time.Now().UTC()

	return nowUTC.After(expiryDateUTC)
}

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[0]
}

func removeDuplicates(elements []record) []record {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []record{}

	for v := range elements {
		if encountered[elements[v].key] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v].key] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}

// GetItems is a function that retuns keys in batch
func (b *DynamoDBBackend) GetItems(path []string) ([]backend.Item, error) {
	fullPath := b.fullPath(path...)
	records, err := b.getRecords(fullPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	values := make([]backend.Item, len(records))
	for i, r := range records {
		values[i] = backend.Item{
			Key:   r.key,
			Value: r.Value,
		}
	}
	return values, nil
}

// GetKeys retrieve all keys matching specific path
func (b *DynamoDBBackend) GetKeys(path []string) ([]string, error) {
	records, err := b.getRecords(b.fullPath(path...))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keys := make([]string, len(records))
	for i, r := range records {
		keys[i] = r.key
	}
	return keys, nil
}

// createKey helper creates a new key/value pair in Dynamo with a given TTL.
// if such key already exists, it:
// 	   overwrites it if 'overwrite' is true
//     atomically returns AlreadyExists error if 'overwrite' is false
func (b *DynamoDBBackend) createKey(fullPath string, val []byte, ttl time.Duration, overwrite bool) error {
	r := record{
		HashKey:   hashKey,
		FullPath:  fullPath,
		Value:     val,
		TTL:       ttl,
		Timestamp: time.Now().UTC().Unix(),
	}
	if ttl != backend.Forever {
		r.Expires = aws.Int64(b.clock.Now().UTC().Add(ttl).Unix())
	}
	av, err := dynamodbattribute.MarshalMap(r)
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(b.Tablename),
	}
	if !overwrite {
		input.SetConditionExpression("attribute_not_exists(FullPath)")
	}
	_, err = b.svc.PutItem(&input)
	err = convertError(err)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateVal create a key with defined value
func (b *DynamoDBBackend) CreateVal(path []string, key string, val []byte, ttl time.Duration) error {
	fullPath := b.fullPath(append(path, key)...)
	return b.createKey(fullPath, val, ttl, false)
}

// UpsertVal update or create a key with defined value (refresh TTL if already exist)
func (b *DynamoDBBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	fullPath := b.fullPath(append(path, key)...)
	return b.createKey(fullPath, val, ttl, true)
}

// CompareAndSwapVal compares and swap values in atomic operation
func (b *DynamoDBBackend) CompareAndSwapVal(path []string, key string, val []byte, prevVal []byte, ttl time.Duration) error {
	if len(prevVal) == 0 {
		return trace.BadParameter("missing prevVal parameter, to atomically create item, use CreateVal method")
	}
	fullPath := b.fullPath(append(path, key)...)
	r := record{
		HashKey:   hashKey,
		FullPath:  fullPath,
		Value:     val,
		TTL:       ttl,
		Timestamp: time.Now().UTC().Unix(),
	}
	if ttl != backend.Forever {
		r.Expires = aws.Int64(b.clock.Now().UTC().Add(ttl).Unix())
	}
	av, err := dynamodbattribute.MarshalMap(r)
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(b.Tablename),
	}
	input.SetConditionExpression("#v = :prev")
	input.SetExpressionAttributeNames(map[string]*string{
		"#v": aws.String("Value"),
	})
	input.SetExpressionAttributeValues(map[string]*dynamodb.AttributeValue{
		":prev": &dynamodb.AttributeValue{
			B: prevVal,
		},
	})
	_, err = b.svc.PutItem(&input)
	err = convertError(err)
	if err != nil {
		// in this case let's use more specific compare failed error
		if trace.IsAlreadyExists(err) {
			return trace.CompareFailed(err.Error())
		}
		return trace.Wrap(err)
	}
	return nil
}

const delayBetweenLockAttempts = 100 * time.Millisecond

// AcquireLock for a token
func (b *DynamoDBBackend) AcquireLock(token string, ttl time.Duration) error {
	val := []byte("lock")
	lockP := b.fullPath("locks", token)

	if err := backend.ValidateLockTTL(ttl); err != nil {
		return trace.Wrap(err)
	}
	for {
		// try reading the lock key. if its TTL is old, it will be deleted:
		b.getKey(lockP)

		// creating a key with overwrite=false is an atomic op:
		err := b.createKey(lockP, val, ttl, false)
		if err == nil {
			// success. lock acquired:
			return nil
		}
		time.Sleep(delayBetweenLockAttempts)
	}
}

// ReleaseLock for a token
func (b *DynamoDBBackend) ReleaseLock(token string) error {
	fp := b.fullPath("locks", token)
	if _, err := b.getKey(fp); err != nil {
		return err
	}
	return b.deleteKey(fp)
}

// DeleteBucket remove all prefixed keys
// WARNING: there is no bucket feature, deleting "bucket" mean a deletion one by one
func (b *DynamoDBBackend) DeleteBucket(path []string, key string) error {
	fullPath := b.fullPath(append(path, key)...)
	query := "HashKey = :hashKey AND begins_with (#K, :fullpath)"
	attrV := map[string]string{":fullpath": fullPath, ":hashKey": hashKey}
	attrN := map[string]*string{"#K": aws.String("FullPath")}
	av, err := dynamodbattribute.MarshalMap(attrV)
	input := dynamodb.QueryInput{
		KeyConditionExpression:    aws.String(query),
		TableName:                 &b.Tablename,
		ExpressionAttributeValues: av, ExpressionAttributeNames: attrN,
	}
	out, err := b.svc.Query(&input)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: manage paginated result
	for _, item := range out.Items {
		var r record
		dynamodbattribute.UnmarshalMap(item, &r)
		if strings.Compare(fullPath, r.FullPath[:len(fullPath)]) == 0 {
			// TODO: bulk delete to optimize
			b.deleteKey(r.FullPath)
		}
	}
	return nil
}

// DeleteKey remove a key
func (b *DynamoDBBackend) DeleteKey(path []string, key string) error {
	fullPath := b.fullPath(append(path, key)...)
	if _, err := b.getKey(fullPath); err != nil {
		return err
	}
	return b.deleteKey(fullPath)
}

func (b *DynamoDBBackend) deleteKey(fullPath string) error {
	av, err := dynamodbattribute.MarshalMap(keyLookup{
		HashKey:  hashKey,
		FullPath: fullPath,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.DeleteItemInput{Key: av, TableName: aws.String(b.Tablename)}
	if _, err = b.svc.DeleteItem(&input); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (b *DynamoDBBackend) getKey(fullPath string) (*record, error) {
	av, err := dynamodbattribute.MarshalMap(keyLookup{
		HashKey:  hashKey,
		FullPath: fullPath,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := dynamodb.GetItemInput{Key: av, TableName: aws.String(b.Tablename)}
	out, err := b.svc.GetItem(&input)
	if err != nil {
		return nil, trace.NotFound("%v not found", fullPath)
	}
	// Item not found, double check if key is a "directory"
	if len(out.Item) == 0 {
		query := "HashKey = :hashKey AND begins_with (#K, :fullpath)"
		attrV := map[string]string{":fullpath": fullPath + "/", ":hashKey": hashKey}
		attrN := map[string]*string{"#K": aws.String("FullPath")}
		av, _ := dynamodbattribute.MarshalMap(attrV)
		input := dynamodb.QueryInput{
			KeyConditionExpression:    aws.String(query),
			TableName:                 &b.Tablename,
			ExpressionAttributeValues: av,
			ExpressionAttributeNames:  attrN,
		}
		out, err := b.svc.Query(&input)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if len(out.Items) > 0 {
			return nil, trace.BadParameter("key is a directory")
		}
		return nil, trace.NotFound("%v not found", fullPath)
	}
	var r record
	dynamodbattribute.UnmarshalMap(out.Item, &r)
	// Check if key expired, if expired delete it
	if r.isExpired() {
		b.deleteKey(fullPath)
		return nil, trace.NotFound("%v not found", fullPath)
	}
	return &r, nil
}

// GetVal retrieve a value from a key
func (b *DynamoDBBackend) GetVal(path []string, key string) ([]byte, error) {
	fullPath := b.fullPath(append(path, key)...)
	r, err := b.getKey(fullPath)
	if err != nil {
		return nil, err
	}
	return r.Value, nil
}

func convertError(err error) error {
	if err == nil {
		return nil
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		return err
	}
	switch aerr.Code() {
	case dynamodb.ErrCodeConditionalCheckFailedException:
		return trace.AlreadyExists(aerr.Error())
	case dynamodb.ErrCodeProvisionedThroughputExceededException:
		return trace.ConnectionProblem(aerr, aerr.Error())
	case dynamodb.ErrCodeResourceNotFoundException:
		return trace.NotFound(aerr.Error())
	case dynamodb.ErrCodeItemCollectionSizeLimitExceededException:
		return trace.BadParameter(aerr.Error())
	case dynamodb.ErrCodeInternalServerError:
		return trace.BadParameter(aerr.Error())
	default:
		return err
	}
}

type records []record

// Len is part of sort.Interface.
func (r records) Len() int {
	return len(r)
}

// Swap is part of sort.Interface.
func (r records) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// Less is part of sort.Interface.
func (r records) Less(i, j int) bool {
	return r[i].key < r[j].key
}
