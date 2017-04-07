/*
Package dynamodbDynamoDBBackend implements DynamoDB storage backend
for Teleport auth service, similar to etcd backend.

dynamo package implements the DynamoDB storage back-end for the
auth server. Originally contributed by https://github.com/apestel

limitations:

* Paging is not implemented, hence all range operations are limited
  to 1MB result set
*/
package dynamo
