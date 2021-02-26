// Copyright 2018 Francisco Souza. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"fmt"

	"cloud.google.com/go/storage"
)

// Object represents the object that is stored within the fake server.
type Object struct {
	BucketName      string `json:"-"`
	Name            string `json:"-"`
	ContentType     string
	ContentEncoding string
	Content         []byte
	Crc32c          string
	Md5Hash         string
	ACL             []storage.ACLRule
	Metadata        map[string]string
	Created         string
	Deleted         string
	Updated         string
	Generation      int64
}

// ID is used for comparing objects.
func (o *Object) ID() string {
	return fmt.Sprintf("%s#%d", o.IDNoGen(), o.Generation)
}

// IDNoGen does not consider the generation field.
func (o *Object) IDNoGen() string {
	return fmt.Sprintf("%s/%s", o.BucketName, o.Name)
}
