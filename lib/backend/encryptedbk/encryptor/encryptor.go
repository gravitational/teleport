/*
Copyright 2015 Gravitational, Inc.

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
package encryptor

type Encryptor interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
}

type Key struct {
	Name         string
	ID           string
	Value        []byte //used in NaCl
	PublicValue  []byte // used in PGP
	PrivateValue []byte // usef in PGP
}

func (key Key) Public() Key {
	pub := Key{
		Name:        key.Name,
		ID:          key.ID,
		PublicValue: key.PublicValue,
	}
	return pub
}

type KeyGenerator func(string) (Key, error)
