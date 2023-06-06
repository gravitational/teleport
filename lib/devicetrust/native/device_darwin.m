// Copyright 2022 Gravitational, Inc
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

#include "device_darwin.h"

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <IOKit/IOKitLib.h>
#import <Security/Security.h>

#include <stdlib.h>
#include <string.h>

const char *kDeviceKeyLabel = "com.gravitational.teleport.devicekey";

const int kErrNoApplicationTag = 1;
const int kErrNoValueRef = 2;
const int kErrCopyPubKeyFailed = 3;
const int kErrIOMatchingServiceFailed = 4;
const int kErrIORegistryEntryFailed = 5;

OSStatus findDeviceKey(NSNumber *retAttrs, CFTypeRef *out) {
  NSData *label = [NSData dataWithBytes:(void *)kDeviceKeyLabel
                                 length:strlen(kDeviceKeyLabel)];
  NSDictionary *query = @{
    (id)kSecClass : (id)kSecClassKey,
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecMatchLimit : (id)kSecMatchLimitOne,
    (id)kSecReturnRef : @YES,
    (id)kSecReturnAttributes : (id)retAttrs,
    (id)kSecAttrApplicationLabel : label,
  };
  return SecItemCopyMatching((CFDictionaryRef)query, out);
}

void copyPublicKey(const char *id, CFDataRef pubKeyRep, PublicKey *pubKeyOut) {
  pubKeyOut->id = strdup(id);
  pubKeyOut->pub_key_len = CFDataGetLength(pubKeyRep);
  pubKeyOut->pub_key = calloc(pubKeyOut->pub_key_len, sizeof(uint8_t));
  memcpy(pubKeyOut->pub_key, CFDataGetBytePtr(pubKeyRep),
         pubKeyOut->pub_key_len);
}

int32_t DeviceKeyGetOrCreate(const char *newID, PublicKey *pubKeyOut) {
  CFErrorRef err = NULL;
  SecAccessControlRef access = NULL;
  NSString *label = NULL;     // managed by ARC
  NSData *labelAsData = NULL; // managed by ARC
  NSData *idAsData = NULL;    // managed by ARC
  NSDictionary *attrs = NULL; // managed by ARC
  SecKeyRef privKey = NULL;
  SecKeyRef pubKey = NULL;
  CFDataRef pubKeyRep = NULL;

  // Unlike other checks, here we only proceed if the device key wasn't found.
  // If DeviceKeyGet is successful, then a device key already exists.
  // If DeviceKeyGet fails unexpectedly, then we pass that error forward and do
  // not proceed.
  int32_t res = DeviceKeyGet(pubKeyOut);
  if (res != errSecItemNotFound) {
    goto end;
  }
  res = 0; // default is that key creation works

  access = SecAccessControlCreateWithFlags(
      kCFAllocatorDefault, kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
      kSecAccessControlPrivateKeyUsage, &err);
  if (!access) {
    res = CFErrorGetCode(err);
    goto end;
  }

  label = [NSString stringWithUTF8String:kDeviceKeyLabel];
  labelAsData = [NSData dataWithBytes:kDeviceKeyLabel
                               length:strlen(kDeviceKeyLabel)];
  idAsData = [NSData dataWithBytes:(void *)newID length:strlen(newID)];
  attrs = @{
    // Secure Enclave requires EC/256bit keys.
    (id)kSecAttrKeyType : (id)kSecAttrKeyTypeECSECPrimeRandom,
    (id)kSecAttrKeySizeInBits : @256,
    (id)kSecAttrTokenID : (id)kSecAttrTokenIDSecureEnclave,

    (id)kSecPrivateKeyAttrs : @{
      (id)kSecAttrIsPermanent : @YES,
      (id)kSecAttrAccessControl : (__bridge id)access,
      // kSecAttrLabel is a human-readable label.
      // CFStringRef
      (id)kSecAttrLabel : label,
      // kSecAttrApplicationLabel is used to lookup keys programatically.
      // CFDataRef
      (id)kSecAttrApplicationLabel : labelAsData,
      // kSecAttrApplicationTag is a private application tag.
      // CFDataRef
      (id)kSecAttrApplicationTag : idAsData,
    },
  };
  privKey = SecKeyCreateRandomKey((__bridge CFDictionaryRef)attrs, &err);
  if (!privKey) {
    res = CFErrorGetCode(err);
    goto end;
  }

  pubKey = SecKeyCopyPublicKey(privKey);
  if (!pubKey) {
    res = kErrCopyPubKeyFailed;
    goto end;
  }

  pubKeyRep = SecKeyCopyExternalRepresentation(pubKey, &err);
  if (!pubKeyRep) {
    res = CFErrorGetCode(err);
    goto end;
  }
  copyPublicKey(newID, pubKeyRep, pubKeyOut);

end:
  if (pubKeyRep) {
    CFRelease(pubKeyRep);
  }
  if (pubKey) {
    CFRelease(pubKey);
  }
  if (privKey) {
    CFRelease(privKey);
  }
  if (access) {
    CFRelease(access);
  }
  if (err) {
    CFRelease(err);
  }
  return res;
}

int32_t DeviceKeyGet(PublicKey *pubKeyOut) {
  CFDictionaryRef attrs = NULL;
  CFDataRef appTagData = NULL; // managed by attrs
  NSString *appTag = NULL;     // managed by ARC
  SecKeyRef privKey = NULL;    // managed by attrs
  SecKeyRef pubKey = NULL;
  CFErrorRef err = NULL;
  CFDataRef pubKeyRep = NULL;

  int32_t res = findDeviceKey(@YES /* retAttrs */, (CFTypeRef *)&attrs);
  if (res != errSecSuccess) {
    goto end;
  }

  appTagData = CFDictionaryGetValue(attrs, kSecAttrApplicationTag);
  if (!appTagData) {
    res = kErrNoApplicationTag;
    goto end;
  }
  appTag = [[NSString alloc] initWithData:(__bridge NSData *)appTagData
                                 encoding:NSUTF8StringEncoding];

  privKey = (SecKeyRef)CFDictionaryGetValue(attrs, kSecValueRef);
  if (!privKey) {
    res = kErrNoValueRef;
    goto end;
  }

  pubKey = SecKeyCopyPublicKey(privKey);
  if (!pubKey) {
    res = kErrCopyPubKeyFailed;
    goto end;
  }

  pubKeyRep = SecKeyCopyExternalRepresentation(pubKey, &err);
  if (!pubKeyRep) {
    res = CFErrorGetCode(err);
    goto end;
  }
  copyPublicKey([appTag UTF8String], pubKeyRep, pubKeyOut);

end:
  if (pubKeyRep) {
    CFRelease(pubKeyRep);
  }
  if (err) {
    CFRelease(err);
  }
  if (pubKey) {
    CFRelease(pubKey);
  }
  if (attrs) {
    CFRelease(attrs);
  }
  return res;
}

int32_t DeviceKeySign(Digest digest, Signature *sigOut) {
  SecKeyRef privKey = NULL;
  NSData *data = NULL; // managed by ARC
  CFErrorRef err = NULL;
  CFDataRef sig = NULL;

  int32_t res = findDeviceKey(@NO /* retAttrs */, (CFTypeRef *)&privKey);
  if (res != errSecSuccess) {
    goto end;
  }

  data = [NSData dataWithBytes:digest.data length:digest.data_len];
  sig = SecKeyCreateSignature(privKey,
                              kSecKeyAlgorithmECDSASignatureDigestX962SHA256,
                              (CFDataRef)data, &err);
  if (!sig) {
    res = CFErrorGetCode(err);
    goto end;
  }

  sigOut->data_len = CFDataGetLength(sig);
  sigOut->data = calloc(sigOut->data_len, sizeof(uint8_t));
  memcpy(sigOut->data, CFDataGetBytePtr(sig), sigOut->data_len);

end:
  if (sig) {
    CFRelease(sig);
  }
  if (err) {
    CFRelease(err);
  }
  if (privKey) {
    CFRelease(privKey);
  }
  return res;
}

// Duplicate a CFString or CFData `ref` as a C string.
const char *refToCString(CFTypeRef ref) {
  NSData *data = NULL;  // managed by ARC.
  NSString *str = NULL; // managed by ARC.
  CFTypeID id;

  if (!ref) {
    return NULL;
  }

  id = CFGetTypeID(ref);
  if (id == CFStringGetTypeID()) {
    str = (__bridge NSString *)ref;
  } else if (id == CFDataGetTypeID()) {
    data = (__bridge NSData *)ref;
    str = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
  } else {
    return NULL;
  }

  return strdup([str UTF8String]);
}

int32_t DeviceCollectData(DeviceData *out) {
  CFMutableDictionaryRef cfIODict = NULL; // manually released
  NSProcessInfo *info = NULL;             // managed by ARC
  NSOperatingSystemVersion osVersion;
  int32_t res = 0;

  io_service_t platformExpert = IOServiceGetMatchingService(
      0 /* mainPort */, IOServiceMatching("IOPlatformExpertDevice"));
  if (!platformExpert) {
    res = kErrIOMatchingServiceFailed;
    goto end;
  }

  // For a quick reference, see `ioreg -c IOPlatformExpertDevice -d 2`.
  IORegistryEntryCreateCFProperties(platformExpert, &cfIODict,
                                    kCFAllocatorDefault, 0 /* options */);
  if (!cfIODict) {
    res = kErrIORegistryEntryFailed;
    goto end;
  }

  // Serial number and model from IORegistry.
  out->serial_number = refToCString(
      CFDictionaryGetValue(cfIODict, CFSTR(kIOPlatformSerialNumberKey)));
  out->model = refToCString(CFDictionaryGetValue(cfIODict, CFSTR("model")));

  // OS version numbers.
  info = [NSProcessInfo processInfo];
  osVersion = [info operatingSystemVersion];
  out->os_version_string =
      strdup([[info operatingSystemVersionString] UTF8String]);
  out->os_major = osVersion.majorVersion;
  out->os_minor = osVersion.minorVersion;
  out->os_patch = osVersion.patchVersion;

end:
  if (cfIODict) {
    CFRelease(cfIODict);
  }
  if (platformExpert) {
    IOObjectRelease(platformExpert);
  }
  return res;
}
