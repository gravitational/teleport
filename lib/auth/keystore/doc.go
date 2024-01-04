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

// Package keystore provides a generic client and associated helpers for handling
// private keys that may be backed by an HSM or KMS.
//
// # Notes on testing
//
// Fully testing the Keystore package predictably requires an HSM. Testcases are
// currently written for the software KeyStore (no HSM), SoftHSMv2, YubiHSM2,
// AWS CloudHSM, and GCP KMS. Only the software tests run without any setup, but
// testing for SoftHSM is enabled by default in the Teleport docker buildbox and
// will be run in CI.
//
// # Testing this package with SoftHSMv2
//
// To test with SoftHSMv2, you must install it (see
// https://github.com/opendnssec/SoftHSMv2 or
// https://packages.ubuntu.com/search?keywords=softhsm2) and set the
// "SOFTHSM2_PATH" environment variable to the location of SoftHSM2's PKCS11
// library. Depending how you installed it, this is likely to be
// /usr/lib/softhsm/libsofthsm2.so or /usr/local/lib/softhsm/libsofthsm2.so.
//
// The test will create its own config file and token, and clean up after itself.
//
// # Testing this package with YubiHSM2
//
// To test with YubiHSM2, you must:
//
// 1. have a physical YubiHSM plugged in
//
// 2. install the SDK (https://developers.yubico.com/YubiHSM2/Releases/)
//
// 3. start the connector "yubihsm-connector -d"
//
//  4. create a config file
//     connector = http://127.0.0.1:12345
//     debug
//
// 5. set "YUBIHSM_PKCS11_CONF" to the location of your config file
//
// 6. set "YUBIHSM_PKCS11_PATH" to the location of the PKCS11 library
//
// The test will use the factory default pin of "0001password" in slot 0.
//
// # Testing this package with AWS CloudHSM
//
// 1. Create a CloudHSM Cluster and HSM, and activate them https://docs.aws.amazon.com/cloudhsm/latest/userguide/getting-started.html
//
// 2. Connect an EC2 instance to the cluster https://docs.aws.amazon.com/cloudhsm/latest/userguide/configure-sg-client-instance.html
//
// 3. Install the CloudHSM client on the EC2 instance https://docs.aws.amazon.com/cloudhsm/latest/userguide/install-and-configure-client-linux.html
//
// 4. Create a Crypto User (CU) https://docs.aws.amazon.com/cloudhsm/latest/userguide/manage-hsm-users.html
//
// 5. Set "CLOUDHSM_PIN" to "<username>:<password>" of your crypto user, eg "TestUser:hunter2"
//
// 6. Run the test on the connected EC2 instance
//
// # Testing this package with GCP CloudHSM
//
// 1. Sign into the Gcloud CLI
//
//  2. Create a keyring
//     ```
//     gcloud kms keyrings create "test" --location global
//     ```
//
//  3. Set GCP_KMS_KEYRING to the name of the keyring you just created
//     ```
//     gcloud kms keyrings list --location global
//     export GCP_KMS_KEYRING=<name from above>
//     ```
//
// 4. Run the unit tests
//
// # Testing Teleport with an HSM-backed CA
//
// Integration tests can be found in integration/hsm. They run with SoftHSM by
// default, manually alter the auth config as necessary to test different HSMs.
package keystore
