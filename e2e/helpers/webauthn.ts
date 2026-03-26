/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import {
  createHash,
  createPrivateKey,
  createPublicKey,
  createSign,
  KeyObject,
  randomBytes,
} from 'crypto';

import { Page } from '@playwright/test';
import { Encoder } from 'cbor-x';

import { webauthnCredentialId, webauthnPrivateKey } from './env';

declare global {
  function __e2eWebAuthn(json: string): Promise<string>;
}

// mockWebAuthn sets up a mock for the WebAuthn API in the browser context in a way that
// is compatible with Chromium, Firefox and WebKit.
export async function mockWebAuthn(page: Page) {
  if (await page.evaluate(() => '__e2eWebAuthn' in self)) {
    return;
  }

  await page.exposeFunction('__e2eWebAuthn', async (requestJSON: string) => {
    const request: WebAuthnRequest = JSON.parse(requestJSON);
    return JSON.stringify(currentDevice.processRequest(request));
  });

  await page.addInitScript(initWebAuthnOverride);
}

function initWebAuthnOverride() {
  // @ts-expect-error polyfill
  self.PublicKeyCredential = class PublicKeyCredential {};
  // @ts-expect-error polyfill
  self.AuthenticatorAttestationResponse = class AuthenticatorAttestationResponse {};
  // @ts-expect-error polyfill
  self.AuthenticatorAssertionResponse = class AuthenticatorAssertionResponse {};

  if (!navigator.credentials) {
    const credentials = {
      create: () => Promise.reject(new Error('WebAuthn not available')),
      get: () => Promise.reject(new Error('WebAuthn not available')),
    };

    Object.defineProperty(navigator, 'credentials', {
      value: Object.create(credentials),
      configurable: true,
    });
  }

  function bufToBase64(buf: ArrayBuffer | Uint8Array) {
    const bytes = buf instanceof ArrayBuffer ? new Uint8Array(buf) : buf;

    let binary = '';
    for (const b of bytes) {
      binary += String.fromCharCode(b);
    }

    return btoa(binary);
  }

  function base64ToBuf(b64: string): ArrayBuffer {
    const binary = atob(b64);
    const bytes = new Uint8Array(binary.length);

    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }

    return bytes.buffer;
  }

  function bufToBase64url(buf: ArrayBuffer | Uint8Array) {
    return bufToBase64(buf)
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
  }

  function base64ToBase64url(b64: string) {
    return b64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  }

  async function callWebAuthn<
    T extends WebAuthnCreateResult | WebAuthnGetResult,
  >(
    type: WebAuthnRequest['type'],
    challenge: ArrayBuffer,
    rpId?: string
  ): Promise<T> {
    return JSON.parse(
      await self.__e2eWebAuthn(
        JSON.stringify({
          type,
          challenge: bufToBase64url(challenge),
          rpId: rpId || location.hostname,
          origin: location.origin,
        } satisfies WebAuthnRequest)
      )
    );
  }

  function buildCredential(
    result: WebAuthnCreateResult | WebAuthnGetResult,
    responseProto:
      | AuthenticatorAttestationResponse
      | AuthenticatorAssertionResponse,
    response: AuthenticatorAttestationResponse | AuthenticatorAssertionResponse
  ): Credential {
    const credential = {
      id: base64ToBase64url(result.credentialId),
      rawId: base64ToBuf(result.credentialId),
      type: 'public-key' as const,
      authenticatorAttachment: 'platform' as const,
      response,
      getClientExtensionResults(): AuthenticationExtensionsClientOutputs {
        return {};
      },
    };

    Object.setPrototypeOf(credential, PublicKeyCredential.prototype);
    Object.setPrototypeOf(credential.response, responseProto);

    return credential;
  }

  const cred = navigator.credentials;
  const proto: CredentialsContainer = Object.getPrototypeOf(cred); // we have to override the prototype because WebKit doesn't allow own property overrides on the instance
  const origCreate = cred.create.bind(cred);
  const origGet = cred.get.bind(cred);

  proto.create = async function (options?: CredentialCreationOptions) {
    if (!options?.publicKey) {
      return origCreate(options);
    }

    const result = await callWebAuthn<WebAuthnCreateResult>(
      'webauthn.create',
      options.publicKey.challenge as ArrayBuffer,
      options.publicKey.rp?.id
    );

    const response: AuthenticatorAttestationResponse = {
      clientDataJSON: base64ToBuf(result.clientDataJSON),
      attestationObject: base64ToBuf(result.attestationObject),
      getTransports() {
        return ['internal'];
      },
      getPublicKey() {
        return base64ToBuf(result.publicKey);
      },
      getPublicKeyAlgorithm() {
        return result.publicKeyAlgorithm;
      },
      getAuthenticatorData() {
        return base64ToBuf(result.authenticatorData);
      },
    };

    return buildCredential(
      result,
      AuthenticatorAttestationResponse.prototype,
      response
    );
  };

  proto.get = async function (options?: CredentialRequestOptions) {
    if (!options?.publicKey) {
      return origGet(options);
    }

    const result = await callWebAuthn<WebAuthnGetResult>(
      'webauthn.get',
      options.publicKey.challenge as ArrayBuffer,
      options.publicKey.rpId
    );

    const response: AuthenticatorAssertionResponse = {
      authenticatorData: base64ToBuf(result.authenticatorData),
      clientDataJSON: base64ToBuf(result.clientDataJSON),
      signature: base64ToBuf(result.signature),
      userHandle: null,
    };

    return buildCredential(
      result,
      AuthenticatorAssertionResponse.prototype,
      response
    );
  };
}

interface WebAuthnRequest {
  type: 'webauthn.create' | 'webauthn.get';
  challenge: string;
  rpId: string;
  origin: string;
}

interface WebAuthnCreateResult {
  credentialId: string;
  authenticatorData: string;
  clientDataJSON: string;
  attestationObject: string;
  publicKey: string;
  publicKeyAlgorithm: number;
}

interface WebAuthnGetResult {
  credentialId: string;
  authenticatorData: string;
  clientDataJSON: string;
  signature: string;
}

// configure cbor-x encoder with the options that are compatible with Teleport
const cbor = new Encoder({
  useRecords: false,
  variableMapSize: true,
  // @ts-expect-error useTag259ForMaps is supported at runtime but missing from the type definitions
  useTag259ForMaps: false,
});

function encodeEC2PublicKeyCOSE(x: Buffer, y: Buffer) {
  return Buffer.from(
    cbor.encode(
      new Map<number, number | Buffer>([
        [1, 2], // kty: EC2
        [3, -7], // alg: ES256
        [-1, 1], // crv: P-256
        [-2, x],
        [-3, y],
      ])
    )
  );
}

function encodeAttestationObject(authData: Buffer) {
  return Buffer.from(
    cbor.encode({
      fmt: 'none',
      attStmt: {},
      authData,
    })
  );
}

/**
 * A fake WebAuthn authenticator that emulates registration (attestation) and
 * authentication (assertion) ceremonies.
 */
export class WebAuthnDevice {
  private readonly credentialIdBuf: Buffer;
  private readonly credentialIdB64: string;
  private readonly privateKey: KeyObject;
  private readonly pubKeyCOSE: Buffer;
  private readonly spkiPublicKeyB64: string;
  private signCount: number = 0;

  public constructor(credentialIdBase64?: string) {
    this.credentialIdBuf = credentialIdBase64
      ? Buffer.from(credentialIdBase64, 'base64')
      : randomBytes(32);
    this.credentialIdB64 =
      credentialIdBase64 || this.credentialIdBuf.toString('base64');

    this.privateKey = createPrivateKey({
      key: Buffer.from(webauthnPrivateKey, 'base64'),
      format: 'der',
      type: 'pkcs8',
    });
    const jwk = this.privateKey.export({ format: 'jwk' });
    const x = Buffer.from(jwk.x!, 'base64url');
    const y = Buffer.from(jwk.y!, 'base64url');
    this.pubKeyCOSE = encodeEC2PublicKeyCOSE(x, y);
    const spkiPubicKey = createPublicKey(this.privateKey).export({
      format: 'der',
      type: 'spki',
    });
    this.spkiPublicKeyB64 = spkiPubicKey.toString('base64');
  }

  public processRequest(
    request: WebAuthnRequest
  ): WebAuthnCreateResult | WebAuthnGetResult {
    // This counter is still local to the worker process; a more accurate
    // emulation would require storing the counter in a temporary file or some
    // other kind of cache. Right now it only triggers a credential clone
    // warning in the logs, so it's not a big deal.
    this.signCount++;

    const clientDataJSON = Buffer.from(
      JSON.stringify({
        type: request.type,
        challenge: request.challenge,
        origin: request.origin,
        crossOrigin: false,
      })
    );

    const rpIdHash = createHash('sha256').update(request.rpId).digest();

    const counter = Buffer.alloc(4);
    counter.writeUInt32BE(this.signCount);

    if (request.type === 'webauthn.create') {
      const authenticatorData = Buffer.concat([
        rpIdHash,
        Buffer.from([0x45]), // UP + UV + AT
        counter,
        Buffer.alloc(16), // aaguid
        Buffer.from([
          this.credentialIdBuf.length >> 8,
          this.credentialIdBuf.length & 0xff,
        ]),
        this.credentialIdBuf,
        this.pubKeyCOSE,
      ]);

      const result: WebAuthnCreateResult = {
        credentialId: this.credentialIdB64,
        authenticatorData: authenticatorData.toString('base64'),
        clientDataJSON: clientDataJSON.toString('base64'),
        attestationObject:
          encodeAttestationObject(authenticatorData).toString('base64'),
        publicKey: this.spkiPublicKeyB64,
        publicKeyAlgorithm: -7,
      };

      return result;
    }

    const authenticatorData = Buffer.concat([
      rpIdHash,
      Buffer.from([0x05]), // UP + UV
      counter,
    ]);

    const signature = createSign('SHA256')
      .update(
        Buffer.concat([
          authenticatorData,
          createHash('sha256').update(clientDataJSON).digest(),
        ])
      )
      .sign(this.privateKey);

    const result: WebAuthnGetResult = {
      credentialId: this.credentialIdB64,
      authenticatorData: authenticatorData.toString('base64'),
      clientDataJSON: clientDataJSON.toString('base64'),
      signature: signature.toString('base64'),
    };

    return result;
  }
}

const defaultDevice = new WebAuthnDevice(webauthnCredentialId);
let currentDevice = defaultDevice;

/**
 * Sets currently used WebAuthn device. From now on, all WebAuthn operations
 * will be performed using this device.
 */
export function setCurrentDevice(device: WebAuthnDevice) {
  currentDevice = device;
}
