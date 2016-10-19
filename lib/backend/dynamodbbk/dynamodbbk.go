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

// Package dynamodbDynamoDBBackend implements DynamoDB powered backend
package dynamodbbk

import (
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
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
	Key       string
	Value     []byte
	Timestamp int64
	TTL       time.Duration
}

type keyLookup struct {
	HashKey string
	Key     string
}

var hashKey = "teleport"

// New returns new instance of Etcd-powered backend
func New(cfg Config) (backend.Backend, error) {
	log.Info("[DynamoDB] Initializing DynamoDB backend")
	if err := cfg.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	b := &DynamoDBBackend{tableName: cfg.Tablename, region: cfg.Region}
	var awsConfig aws.Config
	awsConfig.Region = &b.region
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		creds := credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, "")
		awsConfig.Credentials = creds
	}
	sess, err := session.NewSession(&awsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b.svc = dynamodb.New(sess)
	log.Debug("[DynamoDB] AWS session created")
	return b, nil
}

// Close the DynamoDB driver
func (b *DynamoDBBackend) Close() error {
	return nil
}

func (b *DynamoDBBackend) key(keys ...string) string {
	return strings.Join(append([]string{"teleport"}, keys...), "/")
}

// getKeys retrieve all prefixed keys
// WARNING: there is no bucket feature, retrieving a "bucket" mean a full scan on DynamoDB table
// might be quite resource intensive (take care of read provisioning)
func (b *DynamoDBBackend) getKeys(key string) ([]string, error) {
	var vals []string
	query := "HashKey = :hashKey AND begins_with (#K, :key)"
	attrV := map[string]string{":key": key, ":hashKey": hashKey}
	attrN := map[string]*string{"#K": aws.String("Key")}
	av, err := dynamodbattribute.MarshalMap(attrV)
	input := dynamodb.QueryInput{KeyConditionExpression: aws.String(query), TableName: &b.tableName, ExpressionAttributeValues: av, ExpressionAttributeNames: attrN}
	out, err := b.svc.Query(&input)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: manage paginated result
	for _, item := range out.Items {
		var r record
		dynamodbattribute.UnmarshalMap(item, &r)
		if strings.Compare(key, r.Key[:len(key)]) == 0 && len(key) < len(r.Key) {
			if r.TTL != 0 && time.Unix(r.Timestamp, 0).Add(r.TTL).Before(time.Now().UTC()) {
				b.deleteKey(r.Key)
			} else {
				vals = append(vals, suffix(r.Key[len(key)+1:]))
			}
		}
	}
	return vals, nil
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
	keys, err := b.getKeys(b.key(path...))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Sort(sort.StringSlice(keys))
	keys = removeDuplicates(keys)
	log.Debugf("[DynamoDB] return GetKeys(%s)=%s", path, keys)
	return keys, nil
}

func (b *DynamoDBBackend) createKey(fullPath string, val []byte, ttl time.Duration) error {
	r := record{HashKey: hashKey, Key: fullPath, Value: val, TTL: ttl, Timestamp: time.Now().UTC().Unix()}
	av, err := dynamodbattribute.MarshalMap(r)
	if err != nil {
		return trace.Wrap(err)
	}
	input := dynamodb.PutItemInput{Item: av, TableName: aws.String(b.tableName)}
	_, err = b.svc.PutItem(&input)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateVal create a key with defined value
func (b *DynamoDBBackend) CreateVal(path []string, key string, val []byte, ttl time.Duration) error {
	fullPath := b.key(append(path, key)...)
	return b.createKey(fullPath, val, ttl)
}

// UpsertVal update or create a key with defined value (refresh TTL if already exist)
func (b *DynamoDBBackend) UpsertVal(path []string, key string, val []byte, ttl time.Duration) error {
	return b.CreateVal(path, key, val, ttl)
}

// TouchVal refresh a key
func (b *DynamoDBBackend) TouchVal(path []string, key string, ttl time.Duration) error {
	fullPath := b.key(append(path, key)...)
	r, err := b.getKey(fullPath)
	if err != nil {
		return trace.NotFound("%v not found", fullPath)
	}
	return b.CreateVal(path, key, r.Value, ttl)
}

const delayBetweenLockAttempts = 100 * time.Millisecond

// AcquireLock for a token
func (b *DynamoDBBackend) AcquireLock(token string, ttl time.Duration) error {
	for {
		r, err := b.getKey(b.key("locks", token))
		// Lock not acquired yet
		if err != nil {
			return b.createKey(b.key("locks", token), []byte("lock"), ttl)
			// Lock expired, delete key and acquire
		} else if r != nil && r.TTL != 0 && time.Unix(r.Timestamp, 0).Add(r.TTL).Before(time.Now().UTC()) {
			b.deleteKey(b.key("locks", token))
			b.createKey(b.key("locks", token), []byte("lock"), ttl)
			return nil
		}
		time.Sleep(delayBetweenLockAttempts)
	}
}

// ReleaseLock for a token
func (b *DynamoDBBackend) ReleaseLock(token string) error {
	if _, err := b.getKey(b.key("locks", token)); err != nil {
		return err
	}
	return b.deleteKey(b.key("locks", token))
}

// CompareAndSwap key
func (b *DynamoDBBackend) CompareAndSwap(path []string, key string, val []byte, ttl time.Duration, prevVal []byte) ([]byte, error) {
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
	fullPath := b.key(append(path, key)...)
	query := "HashKey = :hashKey AND begins_with (#K, :key)"
	attrV := map[string]string{":key": fullPath, ":hashKey": hashKey}
	attrN := map[string]*string{"#K": aws.String("Key")}
	av, err := dynamodbattribute.MarshalMap(attrV)
	input := dynamodb.QueryInput{KeyConditionExpression: aws.String(query), TableName: &b.tableName, ExpressionAttributeValues: av, ExpressionAttributeNames: attrN}
	out, err := b.svc.Query(&input)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO: manage paginated result
	for _, item := range out.Items {
		var r record
		dynamodbattribute.UnmarshalMap(item, &r)
		if strings.Compare(fullPath, r.Key[:len(fullPath)]) == 0 {
			// TODO: bulk delete to optimize
			b.deleteKey(r.Key)
		}
	}
	return nil
}

// DeleteKey remove a key
func (b *DynamoDBBackend) DeleteKey(path []string, key string) error {
	fullPath := b.key(append(path, key)...)
	if _, err := b.getKey(fullPath); err != nil {
		return err
	}
	return b.deleteKey(fullPath)
}

func (b *DynamoDBBackend) deleteKey(fullPath string) error {
	av, err := dynamodbattribute.MarshalMap(keyLookup{HashKey: hashKey, Key: fullPath})
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
	av, err := dynamodbattribute.MarshalMap(keyLookup{HashKey: hashKey, Key: fullPath})
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
		query := "HashKey = :hashKey AND begins_with (#K, :key)"
		attrV := map[string]string{":key": fullPath + "/", ":hashKey": hashKey}
		attrN := map[string]*string{"#K": aws.String("Key")}
		av, _ := dynamodbattribute.MarshalMap(attrV)
		input := dynamodb.QueryInput{KeyConditionExpression: aws.String(query), TableName: &b.tableName, ExpressionAttributeValues: av, ExpressionAttributeNames: attrN}
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
	if r.TTL != 0 && time.Unix(r.Timestamp, 0).Add(r.TTL).Before(time.Now().UTC()) {
		b.deleteKey(fullPath)
		return nil, trace.NotFound("%v not found", fullPath)
	}
	return &r, nil
}

// GetVal retrieve a value from a key
func (b *DynamoDBBackend) GetVal(path []string, key string) ([]byte, error) {
	fullPath := b.key(append(path, key)...)
	r, err := b.getKey(fullPath)
	if err != nil {
		return nil, err
	}
	return r.Value, nil
}

// GetValAndTTL retrieve a value and a TTL from a key
func (b *DynamoDBBackend) GetValAndTTL(path []string, key string) ([]byte, time.Duration, error) {
	fullPath := b.key(append(path, key)...)
	r, err := b.getKey(fullPath)
	if err != nil {
		return nil, 0, err
	}
	if r.TTL != 0 {
		r.TTL = time.Unix(r.Timestamp, 0).Add(r.TTL).Sub(time.Now().UTC())
	}
	return r.Value, r.TTL, nil
}

func suffix(key string) string {
	vals := strings.Split(key, "/")
	return vals[0]
}
