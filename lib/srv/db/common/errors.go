/*
Copyright 2021 Gravitational, Inc.

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

package common

import (
	"net/http"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/pkg/errors"
	"github.com/siddontang/go-mysql/mysql"
	"google.golang.org/api/googleapi"
	"google.golang.org/grpc/status"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
)

// ConvertError converts errors to trace errors.
func ConvertError(err error) error {
	if err == nil {
		return nil
	}
	// Unwrap original error first.
	if _, ok := err.(*trace.TraceErr); ok {
		return ConvertError(trace.Unwrap(err))
	}
	if pgErr, ok := err.(pgError); ok {
		return ConvertError(pgErr.Unwrap())
	}
	if _, ok := err.(causer); ok {
		return ConvertError(errors.Cause(err))
	}
	if _, ok := status.FromError(err); ok {
		return trail.FromGRPC(err)
	}
	switch e := trace.Unwrap(err).(type) {
	case *googleapi.Error:
		return convertGCPError(e)
	case awserr.RequestFailure:
		return awslib.ConvertRequestFailureError(e)
	case *pgconn.PgError:
		return convertPostgresError(e)
	case *mysql.MyError:
		return convertMySQLError(e)
	}
	return err // Return unmodified.
}

// convertGCPError converts GCP errors to trace errors.
func convertGCPError(err *googleapi.Error) error {
	switch err.Code {
	case http.StatusForbidden:
		return trace.AccessDenied(err.Error())
	case http.StatusConflict:
		return trace.CompareFailed(err.Error())
	}
	return err // Return unmodified.
}

// convertPostgresError converts Postgres driver errors to trace errors.
func convertPostgresError(err *pgconn.PgError) error {
	switch err.Code {
	case pgerrcode.InvalidAuthorizationSpecification, pgerrcode.InvalidPassword:
		return trace.AccessDenied(err.Error())
	}
	return err // Return unmodified.
}

// convertMySQLError converts MySQL driver errors to trace errors.
func convertMySQLError(err *mysql.MyError) error {
	switch err.Code {
	case mysql.ER_ACCESS_DENIED_ERROR:
		return trace.AccessDenied(err.Error())
	}
	return err // Return unmodified.
}

// causer defines an interface for errors wrapped by the "errors" package.
type causer interface {
	Cause() error
}

// pgError defines an interface for errors wrapped by Postgres driver.
type pgError interface {
	Unwrap() error
}
