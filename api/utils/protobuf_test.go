/*
Copyright 2023 Gravitational, Inc.

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

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestCloneProtoMsg(t *testing.T) {
	m := map[string]any{
		"4": 2.0,
		"2": 4.0,
	}
	origMsg, err := structpb.NewStruct(m)
	require.NoError(t, err)

	msgCopy := CloneProtoMsg(origMsg)
	require.Equal(t, origMsg, msgCopy)
	require.IsType(t, origMsg, msgCopy)

	// test that modifying the original doesn't affect the copy
	delete(origMsg.Fields, "2")
	require.Equal(t, m, msgCopy.AsMap())

	// test cloning a nil message
	var sm *structpb.Struct
	smCopy := CloneProtoMsg(sm)
	require.Equal(t, sm, smCopy)
	require.IsType(t, sm, smCopy)
}
