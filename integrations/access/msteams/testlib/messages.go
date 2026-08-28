// Copyright 2024 Gravitational, Inc
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

package testlib

type MsgSlice []Msg
type MsgSet map[Msg]struct{}

func (slice MsgSlice) Len() int {
	return len(slice)
}

func (slice MsgSlice) Less(i, j int) bool {
	return slice[i].RecipientID < slice[j].RecipientID
}

func (slice MsgSlice) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func (set MsgSet) Add(msg Msg) {
	set[msg] = struct{}{}
}

func (set MsgSet) Contains(msg Msg) bool {
	_, ok := set[msg]
	return ok
}
