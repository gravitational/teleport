#!/bin/bash

# required to set AWS_ACL eg. public-read
# required to set S3_BUCKET eg. my-bucket

GIT_REV=$(git rev-parse --verify HEAD)
BINARY_NAME=teleport-${GIT_REV}.tgz
CHECKSUM_NAME=checksums-${GIT_REV}.txt

sha512sum teleport.tgz > $CHECKSUM_NAME
aws s3 cp --acl $AWS_ACL teleport.tgz s3://$S3_BUCKET/$BINARY_NAME
aws s3 cp --acl $AWS_ACL $CHECKSUM_NAME s3://$S3_BUCKET/$CHECKSUM_NAME
