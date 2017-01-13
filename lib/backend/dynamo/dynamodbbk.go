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

dynamo package implements the DynamoDB storage back-end for the
auth server. Originally contributed by https://github.com/apestel

limitations:

- Paging is not implemented, hence all range operations are limited
  to 1MB result set
*/

package dynamo

import (
	"sort"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/gravitational/trace"
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
}

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

	// obsolete schema key. if a table contains "Key" column it means
	// such table needs to be migrated
	oldPathAttr = "Key"

	// conditionFailedErr is an AWS error code for "already exists"
	// when creating a new object:
	conditionFailedErr = "ConditionalCheckFailedException"

	// resourceNotFoundErr is an AWS error code for "resource not found"
	resourceNotFoundErr = "ResourceNotFoundException"
)

// GetName() is a part of backend API and it returns DynamoDB backend type
// as it appears in `storage/type` section of Teleport YAML
func GetName() string {
	return "dynamodb"
}

// New returns new instance of DynamoDB backend.
// It's an implementation of backend API's NewFunc
func New(params backend.Params) (backend.Backend, error) {
	log.Info("[DynamoDB] Initializing DynamoDB backend")

	var cfg *DynamoConfig
	err := utils.ObjectToStruct(params, &cfg)
	if err != nil {
		log.Error(err)
		return nil, trace.BadParameter("DynamoDB configuration is invalid", err)
	}

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
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		creds := credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, "")
		sess.Config.Credentials = creds
	}

	// create DynamoDB service:
	b.svc = dynamodb.New(sess)

	// check if the table exists?
	ts, err := b.getTableStatus(b.tableName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch ts {
	case tableStatusOK:
		break
	case tableStatusMissing:
		err = b.createTable(b.tableName, "FullPath")
	case tableStatusNeedsMigration:
		err = b.migrate(b.tableName)
	}
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

// getTableStatus checks if a given table exists
func (b *DynamoDBBackend) getTableStatus(tableName string) (tableStatus, error) {
	td, err := b.svc.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == resourceNotFoundErr {
				return tableStatusMissing, nil
			}
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
		ReadCapacityUnits:  aws.Int64(5),
		WriteCapacityUnits: aws.Int64(5),
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
	log.Infof("[DynamoDB] waiting until table '%s' is created", tableName)
	err = b.svc.WaitUntilTableExists(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err == nil {
		log.Infof("[DynamoDB] Table '%s' has been created", tableName)
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

// migrate checks if the table contains existing data in "old" format
// which used "Key" and "HashKey" attributes prior to Teleport 1.5
//
// this migration function replaces "Key" with "FullPath":
//     - load all existing entries and keep them in RAM
//     - create <table_name>.bak backup table and copy all entries to it
//     - delete the original table_name
//     - re-create table_name with a new schema (with "FullPath" instead of "Key")
//     - copy all entries to it
func (b *DynamoDBBackend) migrate(tableName string) error {
	backupTableName := tableName + ".bak"
	noMigrationNeededErr := trace.AlreadyExists("table '%s' has already been migrated. see backup in '%s'",
		tableName, backupTableName)

	// make sure migration is needed:
	if status, _ := b.getTableStatus(tableName); status != tableStatusNeedsMigration {
		return trace.Wrap(noMigrationNeededErr)
	}
	// create backup table, or refuse migration if backup table already exists
	s, err := b.getTableStatus(backupTableName)
	if err != nil {
		return trace.Wrap(err)
	}
	if s != tableStatusMissing {
		return trace.Wrap(noMigrationNeededErr)
	}
	log.Infof("[DynamoDB] creating backup table '%s'", backupTableName)
	if err = b.createTable(backupTableName, oldPathAttr); err != nil {
		return trace.Wrap(err)
	}
	log.Infof("[DynamoDB] backup table '%s' created", backupTableName)

	// request all entries in the table (up to 1MB):
	log.Infof("[DynamoDB] pulling legacy records out of '%s'", tableName)
	result, err := b.svc.Query(&dynamodb.QueryInput{
		TableName: aws.String(tableName),
		KeyConditions: map[string]*dynamodb.Condition{
			"HashKey": {
				ComparisonOperator: aws.String(dynamodb.ComparisonOperatorEq),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String("teleport"),
					},
				},
			},
			oldPathAttr: {
				ComparisonOperator: aws.String(dynamodb.ComparisonOperatorBeginsWith),
				AttributeValueList: []*dynamodb.AttributeValue{
					{
						S: aws.String("teleport"),
					},
				},
			},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// copy all items into the backup table:
	log.Infof("[DynamoDB] migrating legacy records to backup table '%s'", backupTableName)
	for _, item := range result.Items {
		_, err = b.svc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(backupTableName),
			Item:      item,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// kill the original table:
	log.Infof("[DynamoDB] deleting legacy table '%s'", tableName)
	if err = b.deleteTable(tableName, true); err != nil {
		log.Warn(err)
	}
	// re-create the original table:
	log.Infof("[DynamoDB] re-creating table '%s' with a new schema", tableName)
	if err = b.createTable(tableName, "FullPath"); err != nil {
		return trace.Wrap(err)
	}
	// copy the items into the new table:
	log.Infof("[DynamoDB] migrating legacy records to the new schema in '%s'", tableName)
	for _, item := range result.Items {
		item["FullPath"] = item["Key"]
		delete(item, "Key")
		_, err = b.svc.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item:      item,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	log.Infof("[DynamoDB] migration succeeded")
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
	// TODO: manage paginated result otherwise only up to 1M (max) of data will be returned.
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
	input := dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(b.tableName),
	}
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
func checkConfig(cfg *DynamoConfig) (err error) {
	// table is not configured?
	if cfg.Tablename == "" {
		return trace.BadParameter("DynamoDB: table_name is not specified")
	}
	return nil
}
