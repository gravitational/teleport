#ifndef DEVICE_DARWIN_H_
#define DEVICE_DARWIN_H_

#include <stddef.h>
#include <stdint.h>

// PublicKey represents a device public key, containing its credential ID and
// binary representation (as returned by SecKeyCopyExternalRepresentation).
// A device key is guaranteed to be stored in the Secure Enclave, and thus is an
// ECDSA P-256 key.
typedef struct PublicKey {
  const char *id;
  uint8_t *pub_key;
  size_t pub_key_len;
} PublicKey;

struct _Bytes {
  uint8_t *data;
  size_t data_len;
};

// Digest is a hash digest to be signed.
typedef struct _Bytes Digest;

// Signature is a signature over a Digest.
typedef struct _Bytes Signature;

// DeviceKeyGetOrCreate creates a new Secure Enclave key for the device, or
// returns the existing key if one is already present.
// Callers are responsible for allocating and freeing all "out" parameters.
// Returns zero for success, non-zero for failures (likely an OSStatus value).
int32_t DeviceKeyGetOrCreate(const char *newID, PublicKey *pubKeyOut);

// DeviceKeyGet returns the current device key or fails.
// Callers are responsible for allocating and freeing all "out" parameters.
// Returns zero for success, non-zero for failures (likely an OSStatus value).
int32_t DeviceKeyGet(PublicKey *pubKeyOut);

// DeviceKeySign signs digest using the device key.
// Callers are responsible for allocating and freeing all "out" parameters.
// Returns zero for success, non-zero for failures (likely an OSStatus value).
int32_t DeviceKeySign(Digest digest, Signature *sigOut);

// DeviceData contains collected data for the device in use.
typedef struct _DeviceData {
  // Mac system serial number.
  // Example: "C02FP3EXXXXX".
  const char *serial_number;
  // Mac device model.
  // See https://support.apple.com/en-us/HT201608.
  // Example: "MacBookPro16,1".
  const char *model;
  // OS version "string", as acquired from NSProcessInfo.
  // Example: "Version 13.4 (Build 22F66)".
  const char *os_version_string;
  int64_t os_major;
  int64_t os_minor;
  int64_t os_patch;
} DeviceData;

// DeviceCollectData collects data for the device in use.
// Callers are responsible for allocating and freeing all "out" parameters.
// Returns zero for success, non-zero for failures.
int32_t DeviceCollectData(DeviceData *out);

#endif // DEVICE_DARWIN_H_
