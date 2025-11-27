/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcputils

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"

	"github.com/gravitational/teleport/lib/utils"
)

// IsOKCloseError checks if provided error is a common close error that
// indicates the connection is ended.
func IsOKCloseError(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) ||
		isFileClosedError(err) ||
		utils.IsOKNetworkError(err) ||
		errors.Is(err, context.Canceled)
}

// isFileClosedError check if the error is a common error when exec.Command
// pipes are closed.
func isFileClosedError(err error) bool {
	var pathErr *fs.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	return errors.Is(pathErr.Err, fs.ErrClosed)
}

type readerParseError struct {
	Err error
}

func (e *readerParseError) Error() string {
	return e.Err.Error()
}

func (e *readerParseError) Unwrap() error {
	return e.Err
}

func isReaderParseError(err error) bool {
	var parseError *readerParseError
	return errors.As(err, &parseError)
}

func newReaderParseError(err error) error {
	return &readerParseError{
		Err: err,
	}
}

// WireError represents a structured error in a Response.
// Copied from https://github.com/modelcontextprotocol/go-sdk/blob/main/internal/jsonrpc2/wire.go
type WireError struct {
	// Code is an error code indicating the type of failure.
	Code int64 `json:"code"`
	// Message is a short description of the error.
	Message string `json:"message"`
	// Data is optional structured data containing additional information about the error.
	Data json.RawMessage `json:"data,omitempty"`
}

func (err *WireError) Error() string {
	return err.Message
}

const (
	ErrCodeParseError int64 = -32700
	ErrCodeInternal   int64 = -32603
)
