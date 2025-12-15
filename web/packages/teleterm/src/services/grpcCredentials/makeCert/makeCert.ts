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

import { promisify } from 'util';

import { md, pki } from 'node-forge';

const generateKeyPair = promisify(pki.rsa.generateKeyPair.bind(pki.rsa));

interface GeneratedCert {
  key: string;
  cert: string;
}

/**
 * Creates a self-signed cert. commonName should be a valid domain name.
 */
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
    {
      name: 'subjectAltName',
      altNames: [
        {
          type: 2, // DNS type
          value: commonName,
        },
      ],
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
