// Copyright 2018 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package backend proides the backends used by fake-gcs-server.
package backend

// Storage is the generic interface for implementing the backend storage of the
// server.
type Storage interface {
	CreateBucket(name string, versioningEnabled bool) error
	ListBuckets() ([]Bucket, error)
	GetBucket(name string) (Bucket, error)
	CreateObject(obj Object) (Object, error)
	ListObjects(bucketName string, versions bool) ([]Object, error)
	GetObject(bucketName, objectName string) (Object, error)
	GetObjectWithGeneration(bucketName, objectName string, generation int64) (Object, error)
	DeleteObject(bucketName, objectName string) error
	PatchObject(bucketName, objectName string, metadata map[string]string) (Object, error)
}
