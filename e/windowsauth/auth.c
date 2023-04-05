#include "auth.h"

typedef unsigned short WCHAR;
typedef WCHAR *PWCHAR, *LPWCH, *PWCH;
typedef WCHAR *NWPSTR, *LPWSTR, *PWSTR;

typedef long NTSTATUS;

typedef unsigned long DWORD;
typedef unsigned long ULONG;
typedef int HRESULT;
typedef int BOOL;

void *allocateLsa(PLSA_DISPATCH_TABLE tbl, ULONG size) {
  return tbl->AllocateLsaHeap(size);
}

void *allocatePrivate(PLSA_DISPATCH_TABLE tbl, SIZE_T size) {
  return tbl->AllocatePrivateHeap(size);
}

NTSTATUS allocateClient(PLSA_DISPATCH_TABLE tbl, PLSA_CLIENT_REQUEST req,
                        ULONG size, void **out) {
  return tbl->AllocateClientBuffer(req, size, out);
}

NTSTATUS copyToClientBuffer(PLSA_DISPATCH_TABLE tbl,
                            PLSA_CLIENT_REQUEST ClientRequest, ULONG Length,
                            PVOID ClientBaseAddress, void *BufferToCopy) {
  return tbl->CopyToClientBuffer(ClientRequest, Length, ClientBaseAddress,
                                 BufferToCopy);
}

NTSTATUS createLogonSession(PLSA_DISPATCH_TABLE tbl, PLUID LogonId) {
  return tbl->CreateLogonSession(LogonId);
}

void *hInstance;

__declspec(dllexport) int DllMain(void *hModule,
                                  unsigned long ul_reason_for_call,
                                  void *lpReserved) {
  hInstance = hModule;
  return 1;
}
