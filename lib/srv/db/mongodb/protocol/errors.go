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

package protocol

import (
	"net"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"

	"github.com/gravitational/trace"
)

// ReplyError sends error wire message to the client.
func ReplyError(clientConn net.Conn, replyTo Message, clientErr error) (err error) {
	if msgCompressed, ok := replyTo.(*MessageOpCompressed); ok {
		replyTo = msgCompressed.GetOriginal()
	}
	var errMessage Message
	switch replyTo.(type) {
	case *MessageOpMsg: // When client request is OP_MSG, reply should be OP_MSG as well.
		errMessage, err = makeOpMsgError(clientErr)
	default: // Send OP_REPLY otherwise.
		errMessage, err = makeOpReplyError(clientErr)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	// Encode request ID in the response's "replyTo" header field since the
	// client may check which request this error response corresponds to.
	var replyToID int32
	if replyTo != nil {
		replyToID = replyTo.GetHeader().RequestID
	}
	_, err = clientConn.Write(errMessage.ToWire(replyToID))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// makeOpReplyError builds a OP_REPLY error wire message.
func makeOpReplyError(err error) (Message, error) {
	document, err := bson.Marshal(bson.M{
		"$err": err.Error(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return MakeOpReplyWithFlags(document, wiremessage.QueryFailure), nil
}

// makeOpMsgError builds a OP_MSG error wire message.
func makeOpMsgError(err error) (Message, error) {
	document, err := bson.Marshal(bson.M{
		"ok":     0,
		"errmsg": err.Error(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return MakeOpMsg(document), nil
}
