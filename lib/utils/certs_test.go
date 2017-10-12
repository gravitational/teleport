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

package utils

import (
	"io/ioutil"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

type CertsSuite struct {
}

var _ = Suite(&CertsSuite{})

func (_ *CertsSuite) TestRejectsInvalidPEMData(c *C) {
	_, err := ReadCertificateChain([]byte("no data"))
	c.Assert(trace.Unwrap(err), FitsTypeOf, &trace.NotFoundError{})
}

func (_ *CertsSuite) TestRejectsSelfSignedCertificate(c *C) {
	certificateChainBytes, err := ioutil.ReadFile("../../fixtures/certs/ca.pem")
	c.Assert(err, IsNil)

	certificateChain, err := ReadCertificateChain(certificateChainBytes)
	c.Assert(err, IsNil)

	err = VerifyCertificateChain(certificateChain)
	c.Assert(err, ErrorMatches, "x509: certificate signed by unknown authority")
}
