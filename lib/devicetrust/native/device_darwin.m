// go:build darwin

#include "device_darwin.h"

int64_t DeviceKeyCreate(const char *newID, PublicKey *pubKeyOut) { return -1; }

int64_t DeviceKeyGet(PublicKey *pubKeyOut) { return -1; }

int64_t DeviceKeySign(Digest digest, Signature *sigOut) { return -1; }

int64_t DeviceCollectData(DeviceData *out) { return -1; }
