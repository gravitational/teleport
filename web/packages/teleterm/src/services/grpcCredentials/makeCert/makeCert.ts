/**
 * (The MIT License)
 *
 * Copyright (c) 2019 Subash Pathak <subash@subash.me>
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * 'Software'), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be
 * included in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED 'AS IS', WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

/**
 Copyright 2022 Gravitational, Inc.

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

import { pki, md } from 'node-forge';
import { promisify } from 'util';

const generateKeyPair = promisify(pki.rsa.generateKeyPair.bind(pki.rsa));

interface GeneratedCert {
  key: string;
  cert: string;
}

export async function makeCert({
  commonName,
  validityDays,
}: {
  commonName: string;
  validityDays: number;
}): Promise<GeneratedCert> {
  // certificate Attributes: https://git.io/fptna
  const attributes = [{ name: 'commonName', value: commonName }];

  // required certificate extensions for a certificate authority
  const extensions = [
    { name: 'basicConstraints', cA: true, critical: true },
    {
      name: 'keyUsage',
      keyCertSign: true,
      critical: true,
      digitalSignature: true,
      keyEncipherment: true,
    },
  ];

  return await generateRawCert({
    subject: attributes,
    issuer: attributes,
    extensions,
    validityDays,
  });
}

async function generateRawCert({
  subject,
  issuer,
  extensions,
  validityDays,
  signWith,
}: {
  subject: pki.CertificateField[];
  issuer: pki.CertificateField[];
  extensions: any[];
  validityDays: number;
  signWith?: string;
}): Promise<GeneratedCert> {
  const keyPair = await generateKeyPair({ bits: 2048, workers: 4 });
  const cert = pki.createCertificate();

  cert.publicKey = keyPair.publicKey;
  cert.serialNumber = '0';
  cert.validity.notBefore = new Date();
  cert.validity.notAfter = new Date();
  cert.validity.notAfter.setDate(
    cert.validity.notAfter.getDate() + validityDays
  );
  cert.setSubject(subject);
  cert.setIssuer(issuer);
  cert.setExtensions(extensions);

  // sign the certificate with its own private key if no separate signing key is provided
  const privateKey = signWith
    ? pki.privateKeyFromPem(signWith)
    : keyPair.privateKey;
  cert.sign(privateKey, md.sha256.create());

  return {
    key: pki.privateKeyToPem(keyPair.privateKey),
    cert: pki.certificateToPem(cert),
  };
}
