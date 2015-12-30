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
// package auth implements certificate signing authority and access control server
// Authority server is composed of several parts:
//
// * Authority server itself that implements signing and acl logic
// * HTTP server wrapper for authority server
// * HTTP client wrapper
//

package client

import (
	"io/ioutil"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gokyle/hotp"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
)

// HOTPMock is a HOTP that can be saved or load from file
// Using HOTPMock disables the hotp security level, don't use it in production
type HOTPMock struct {
	*hotp.HOTP
}

func CreateHOTPMock(hotpURLString string) (*HOTPMock, error) {
	otp, _, err := hotp.FromURL(hotpURLString)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &HOTPMock{
		HOTP: otp,
	}, nil
}

func (otp *HOTPMock) SaveToFile(path string) error {
	tokenBytes, err := hotp.Marshal(otp.HOTP)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ioutil.WriteFile(path, tokenBytes, 0666)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func LoadHOTPMockFromFile(path string) (*HOTPMock, error) {
	tokenBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otp, err := hotp.Unmarshal(tokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &HOTPMock{
		HOTP: otp,
	}, nil
}

// GetTokenFromHOTPMockFile opens HOTPMock from file, gets token value,
// increases hotp and saves it to the file. Returns hotp token value.
func GetTokenFromHOTPMockFile(path string) (token string, e error) {
	otp, err := LoadHOTPMockFromFile(path)
	if err != nil {
		return "", trace.Wrap(err)
	}

	token = otp.OTP()

	err = otp.SaveToFile(path)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return token, nil
}
