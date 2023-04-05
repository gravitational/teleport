#include "auth.h"
#include <string.h>

PVOID allocateLsaHeap(ULONG length) { return calloc(1, length); }

LSA_DISPATCH_TABLE tab = {
    .AllocateLsaHeap = allocateLsaHeap,
};

PLSA_DISPATCH_TABLE ptab = &tab;
