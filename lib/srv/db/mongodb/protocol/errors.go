/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package protocol

import (
	"net"

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
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
