// +build dynamodb

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
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gravitational/trace"
)

// DynamoDBBackend struct
type DynamoDBBackend struct {
	tableName string
	region    string
	svc       *dynamodb.DynamoDB
}

type record struct {
	HashKey   string
	FullPath  string
	Value     []byte
	Timestamp int64
	TTL       time.Duration
}

type keyLookup struct {
	HashKey  string
	FullPath string
}

const (
	// hashKey is actually the name of the partition. This backend
	// places all objects in the same DynamoDB partition
	hashKey = "teleport"

	// conditionFailedErr is an AWS error code for "already exists"
	// when creating a new object:
	conditionFailedErr = "ConditionalCheckFailedException"
)

// New returns new instance of Etcd-powered backend
func New(cfg *backend.Config) (*DynamoDBBackend, error) {
	log.Info("[DynamoDB] Initializing DynamoDB backend")
	defer log.Debug("[DynamoDB] AWS session created")

	if err := checkConfig(cfg); err != nil {
		return nil, trace.Wrap(err)
	}
	b := &DynamoDBBackend{
		tableName: cfg.Tablename,
		region:    cfg.Region,
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
	if cfg.DynamoConfig.AccessKey != "" || cfg.DynamoConfig.SecretKey != "" {
		creds := credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, "")
		sess.Config.Credentials = creds
	}
	// create DynamoDB service:
	b.svc = dynamodb.New(sess)
	if err = b.checkOrCreateTable(); err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

// check if table already exist in DynamoDB. If not create it
func (b *DynamoDBBackend) checkOrCreateTable() error {
	lt := dynamodb.ListTablesInput{ExclusiveStartTableName: aws.String(b.tableName)}
	r, err := b.svc.ListTables(&lt)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, t := range r.TableNames {
		if *t == b.tableName {
			log.Info("[DynamoDB] Table already created")
			return nil
		}
	}
	pThroughput := dynamodb.ProvisionedThroughput{
		ReadCapacityUnits:  aws.Int64(5),
		WriteCapacityUnits: aws.Int64(5),
	}
	def := []*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("HashKey"),
			AttributeType: aws.String("S"),
		},
		{
			AttributeName: aws.String("FullPath"),
			AttributeType: aws.String("S"),
		},
	}
	elems := []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("HashKey"),
			KeyType:       aws.String("HASH"),
		},
		{
			AttributeName: aws.String("FullPath"),
			KeyType:       aws.String("RANGE"),
		},
	}
	c := dynamodb.CreateTableInput{
		TableName:             aws.String(b.tableName),
		AttributeDefinitions:  def,
		KeySchema:             elems,
		ProvisionedThroughput: &pThroughput,
	}
	_, err = b.svc.CreateTable(&c)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Info("[DynamoDB] Wait until table is created")
	err = b.svc.WaitUntilTableExists(&dynamodb.DescribeTableInput{
		TableName: aws.String(b.tableName),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	log.Info("[DynamoDB] Table created")
	return nil
}

// Close the DynamoDB driver
func (b *DynamoDBBackend) Close() error {
	return nil
}

func (b *DynamoDBBackend) fullPath(bucket ...string) string {
	return strings.Join(append([]string{"teleport"}, bucket...), "/")
}

// getKeys retrieve all prefixed keys
// WARNING: there is no bucket feature, retrieving a "bucket" mean a full scan on DynamoDB table
// might be quite resource intensive (take care of read provisioning)
func (b *DynamoDBBackend) getKeys(path string) ([]string, error) {
	var vals []string
	query := "HashKey = :hashKey AND begins_with (#K, :fullpath)"
	attrV := map[string]string{":fullpath": path, ":hashKey": hashKey}
	attrN := map[string]*string{"#K": aws.String("FullPath")}
	av, err := dynamodbattribute.MarshalMap(attrV)
	input := dynamodb.QueryInput{
		KeyConditionExpression:    aws.String(query),
		TableName:                 &b.tableName,
		ExpressionAttributeValues: av,
		ExpressionAttributeNames:  attrN,
	}
	out, err := b.svc.Query(&input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// TODO: manage paginated result
	for _, item := range out.Items {
		var r record
		dynamodbattribute.UnmarshalMap(item, &r)

		if strings.Compare(path, r.FullPath[:len(path)]) == 0 && len(path) < len(r.FullPath) {
			if r.isExpired() {
				b.deleteKey(r.FullPath)
			} else {
				vals = append(vals, suffix(r.FullPath[len(path)+1:]))
			}
		}
	}
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

func removeDuplicates(elements []string) []string {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if encountered[elements[v]] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}

// GetKeys retrieve all keys matching specific path
func (b *DynamoDBBackend) GetKeys(path []string) ([]string, error) {
	log.Debugf("[DynamoDB] call GetKeys(%s)", path)
	keys, err := b.getKeys(b.fullPath(path...))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Sort(sort.StringSlice(keys))
	keys = removeDuplicates(keys)
	log.Debugf("[DynamoDB] return GetKeys(%s)=%s", path, keys)
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
	av, err := dynamodbattribute.MarshalMap(r)
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{Item: av, TableName: aws.String(b.tableName)}
	if !overwrite {
		input.SetConditionExpression("attribute_not_exists(FullPath)")
	}
	_, err = b.svc.PutItem(&input)
	if err != nil {
		// special handling for 'already exists':
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == conditionFailedErr {
				return trace.AlreadyExists("%s already exists", fullPath)
			}
		}
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
	return trace.Errorf("failed to acquire lock '%s'", token)
}

// ReleaseLock for a token
func (b *DynamoDBBackend) ReleaseLock(token string) error {
	fp := b.fullPath("locks", token)
	if _, err := b.getKey(fp); err != nil {
		return err
	}
	return b.deleteKey(fp)
}

// CompareAndSwap key
func (b *DynamoDBBackend) CompareAndSwap(
	path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {

	storedVal, err := b.GetVal(path, key)
	if err != nil {
		if trace.IsNotFound(err) && len(prevVal) != 0 {
			return nil, err
		}
	}
	if len(prevVal) == 0 && err == nil {
		return nil, trace.AlreadyExists("key '%v' already exists", key)
	}
	if string(prevVal) == string(storedVal) {
		err = b.UpsertVal(path, key, val, ttl)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return storedVal, nil
	}
	return storedVal, trace.CompareFailed("expected: %v, got: %v", string(prevVal), string(storedVal))
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
		TableName:                 &b.tableName,
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
	input := dynamodb.DeleteItemInput{Key: av, TableName: aws.String(b.tableName)}
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
	input := dynamodb.GetItemInput{Key: av, TableName: aws.String(b.tableName)}
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
			TableName:                 &b.tableName,
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

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[0]
}

// checkConfig helper returns an error if the supplied configuration
// is not enough to connect to DynamoDB
func checkConfig(cfg *backend.Config) (err error) {
	// table is not configured?
	if cfg.Tablename == "" {
		return trace.BadParameter("DynamoDB: table_name is not specified")
	}
	return nil
}
