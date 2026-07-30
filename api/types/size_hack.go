// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/runtime/protoiface"
)

func init() {
	protoadapt.MessageV2Of((*RoleV6)(nil)).ProtoReflect().ProtoMethods().Size = sizeHackForRoleV6
}

func sizeHackForRoleV6(in protoiface.SizeInput) protoiface.SizeOutput {
	return protoiface.SizeOutput{
		Size: protoadapt.MessageV1Of(in.Message.Interface()).(*RoleV6).Size(),
	}
}
