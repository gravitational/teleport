#ifndef AUTH_H
#define AUTH_H

#include <stdlib.h>

typedef unsigned long DWORD;
typedef unsigned short USHORT;
typedef long LONG, *PLONG;
typedef unsigned long ULONG, *PULONG;
typedef char CHAR, *PCHAR;
typedef unsigned char BOOLEAN, UCHAR, *PUCHAR;
typedef size_t SIZE_T;

typedef long NTSTATUS;
typedef void VOID, *PVOID;
typedef PVOID *PLSA_CLIENT_REQUEST;

typedef struct _LUID {
  DWORD LowPart;
  LONG HighPart;
} LUID, *PLUID;

typedef unsigned short WCHAR;
typedef WCHAR *PWCHAR, *LPWCH, *PWCH;
typedef WCHAR *NWPSTR, *LPWSTR, *PWSTR;

typedef struct _STRING {
  USHORT Length;
  USHORT MaximumLength;
  PCHAR Buffer;
} LSA_STRING, *PLSA_STRING;

typedef struct _UNICODE_STRING {
  USHORT Length;
  USHORT MaximumLength;
  PWSTR Buffer;
} UNICODE_STRING, *PUNICODE_STRING;

#define TOKEN_SOURCE_LENGTH 8

typedef struct _TOKEN_SOURCE {
  CHAR SourceName[TOKEN_SOURCE_LENGTH];
  LUID SourceIdentifier;
} TOKEN_SOURCE, *PTOKEN_SOURCE;

typedef NTSTATUS(LSA_CREATE_LOGON_SESSION)(PLUID LogonId);
typedef PVOID(LSA_ALLOCATE_LSA_HEAP)(ULONG Length);
typedef PVOID(LSA_ALLOCATE_PRIVATE_HEAP)(SIZE_T Length);
typedef NTSTATUS(LSA_ALLOCATE_CLIENT_BUFFER)(PLSA_CLIENT_REQUEST ClientRequest,
                                             ULONG LengthRequired,
                                             PVOID *ClientBaseAddress);
typedef NTSTATUS(LSA_COPY_TO_CLIENT_BUFFER)(PLSA_CLIENT_REQUEST ClientRequest,
                                            ULONG Length,
                                            PVOID ClientBaseAddress,
                                            PVOID BufferToCopy);
typedef LSA_CREATE_LOGON_SESSION *PLSA_CREATE_LOGON_SESSION;
typedef LSA_ALLOCATE_LSA_HEAP *PLSA_ALLOCATE_LSA_HEAP;
typedef LSA_ALLOCATE_PRIVATE_HEAP *PLSA_ALLOCATE_PRIVATE_HEAP;
typedef LSA_ALLOCATE_CLIENT_BUFFER *PLSA_ALLOCATE_CLIENT_BUFFER;
typedef LSA_COPY_TO_CLIENT_BUFFER *PLSA_COPY_TO_CLIENT_BUFFER;

typedef struct _LSA_DISPATCH_TABLE {
  PLSA_CREATE_LOGON_SESSION CreateLogonSession;
  void *Dummy1[4];
  PVOID (*AllocateLsaHeap)(ULONG Length);
  void *Dummy2;
  PLSA_ALLOCATE_CLIENT_BUFFER AllocateClientBuffer;
  void *Dummy3;
  PLSA_COPY_TO_CLIENT_BUFFER CopyToClientBuffer;
  void *Dummy4[38];
  PLSA_ALLOCATE_PRIVATE_HEAP AllocatePrivateHeap;
} LSA_DISPATCH_TABLE, *PLSA_DISPATCH_TABLE;

#endif
