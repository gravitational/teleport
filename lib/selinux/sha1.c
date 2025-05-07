///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  LibSha1
//
//  Implementation of SHA1 hash function.
//  Original author:  Steve Reid <sreid@sea-to-sky.net>
//  Contributions by: James H. Brown <jbrown@burgoyne.com>, Saul Kravitz <Saul.Kravitz@celera.com>,
//  and Ralph Giles <giles@ghostscript.com>
//  Modified by WaterJuice retaining Public Domain license.
//
//  This is free and unencumbered software released into the public domain - June 2013 waterjuice.org
//  Modified to:
//    - stop symbols being exported for libselinux shared library - October 2015
//								       Richard Haines <richard_c_haines@btinternet.com>
//    - Not cast the workspace from a byte array to a CHAR64LONG16 due to alignment issues.
//      Fixes:
//        sha1.c:73:33: error: cast from 'uint8_t *' (aka 'unsigned char *') to 'CHAR64LONG16 *' increases required alignment from 1 to 4 [-Werror,-Wcast-align]
//             CHAR64LONG16*       block = (CHAR64LONG16*) workspace;
//                                                                     William Roberts <william.c.roberts@intel.com>
//    - Silence clang's -Wextra-semi-stmt warning - July 2021, Nicolas Iooss <nicolas.iooss@m4x.org>
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  IMPORTS
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

#include "sha1.h"
#include <memory.h>

#include "selinux_internal.h"

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  TYPES
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

typedef union
{
    uint8_t     c [64];
    uint32_t    l [16];
} CHAR64LONG16;

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  INTERNAL FUNCTIONS
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

#define rol(value, bits) (((value) << (bits)) | ((value) >> (32 - (bits))))

// blk0() and blk() perform the initial expand.
#define blk0(i) (block->l[i] = (rol(block->l[i],24)&0xFF00FF00) \
    |(rol(block->l[i],8)&0x00FF00FF))

#define blk(i) (block->l[i&15] = rol(block->l[(i+13)&15]^block->l[(i+8)&15] \
    ^block->l[(i+2)&15]^block->l[i&15],1))

// (R0+R1), R2, R3, R4 are the different operations used in SHA1
#define R0(v,w,x,y,z,i)  do { z += ((w&(x^y))^y)     + blk0(i)+ 0x5A827999 + rol(v,5); w=rol(w,30); } while (0)
#define R1(v,w,x,y,z,i)  do { z += ((w&(x^y))^y)     + blk(i) + 0x5A827999 + rol(v,5); w=rol(w,30); } while (0)
#define R2(v,w,x,y,z,i)  do { z += (w^x^y)           + blk(i) + 0x6ED9EBA1 + rol(v,5); w=rol(w,30); } while (0)
#define R3(v,w,x,y,z,i)  do { z += (((w|x)&y)|(w&x)) + blk(i) + 0x8F1BBCDC + rol(v,5); w=rol(w,30); } while (0)
#define R4(v,w,x,y,z,i)  do { z += (w^x^y)           + blk(i) + 0xCA62C1D6 + rol(v,5); w=rol(w,30); } while (0)


///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  TransformFunction
//
//  Hash a single 512-bit block. This is the core of the algorithm
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
ignore_unsigned_overflow_
static
void
    TransformFunction
    (
        uint32_t            state[5],
        const uint8_t       buffer[64]
    )
{
    uint32_t            a;
    uint32_t            b;
    uint32_t            c;
    uint32_t            d;
    uint32_t            e;
    CHAR64LONG16        workspace;
    CHAR64LONG16*       block = &workspace;

    memcpy(block, buffer, 64);

    // Copy context->state[] to working vars
    a = state[0];
    b = state[1];
    c = state[2];
    d = state[3];
    e = state[4];

    // 4 rounds of 20 operations each. Loop unrolled.
    R0(a,b,c,d,e, 0); R0(e,a,b,c,d, 1); R0(d,e,a,b,c, 2); R0(c,d,e,a,b, 3);
    R0(b,c,d,e,a, 4); R0(a,b,c,d,e, 5); R0(e,a,b,c,d, 6); R0(d,e,a,b,c, 7);
    R0(c,d,e,a,b, 8); R0(b,c,d,e,a, 9); R0(a,b,c,d,e,10); R0(e,a,b,c,d,11);
    R0(d,e,a,b,c,12); R0(c,d,e,a,b,13); R0(b,c,d,e,a,14); R0(a,b,c,d,e,15);
    R1(e,a,b,c,d,16); R1(d,e,a,b,c,17); R1(c,d,e,a,b,18); R1(b,c,d,e,a,19);
    R2(a,b,c,d,e,20); R2(e,a,b,c,d,21); R2(d,e,a,b,c,22); R2(c,d,e,a,b,23);
    R2(b,c,d,e,a,24); R2(a,b,c,d,e,25); R2(e,a,b,c,d,26); R2(d,e,a,b,c,27);
    R2(c,d,e,a,b,28); R2(b,c,d,e,a,29); R2(a,b,c,d,e,30); R2(e,a,b,c,d,31);
    R2(d,e,a,b,c,32); R2(c,d,e,a,b,33); R2(b,c,d,e,a,34); R2(a,b,c,d,e,35);
    R2(e,a,b,c,d,36); R2(d,e,a,b,c,37); R2(c,d,e,a,b,38); R2(b,c,d,e,a,39);
    R3(a,b,c,d,e,40); R3(e,a,b,c,d,41); R3(d,e,a,b,c,42); R3(c,d,e,a,b,43);
    R3(b,c,d,e,a,44); R3(a,b,c,d,e,45); R3(e,a,b,c,d,46); R3(d,e,a,b,c,47);
    R3(c,d,e,a,b,48); R3(b,c,d,e,a,49); R3(a,b,c,d,e,50); R3(e,a,b,c,d,51);
    R3(d,e,a,b,c,52); R3(c,d,e,a,b,53); R3(b,c,d,e,a,54); R3(a,b,c,d,e,55);
    R3(e,a,b,c,d,56); R3(d,e,a,b,c,57); R3(c,d,e,a,b,58); R3(b,c,d,e,a,59);
    R4(a,b,c,d,e,60); R4(e,a,b,c,d,61); R4(d,e,a,b,c,62); R4(c,d,e,a,b,63);
    R4(b,c,d,e,a,64); R4(a,b,c,d,e,65); R4(e,a,b,c,d,66); R4(d,e,a,b,c,67);
    R4(c,d,e,a,b,68); R4(b,c,d,e,a,69); R4(a,b,c,d,e,70); R4(e,a,b,c,d,71);
    R4(d,e,a,b,c,72); R4(c,d,e,a,b,73); R4(b,c,d,e,a,74); R4(a,b,c,d,e,75);
    R4(e,a,b,c,d,76); R4(d,e,a,b,c,77); R4(c,d,e,a,b,78); R4(b,c,d,e,a,79);

    // Add the working vars back into context.state[]
    state[0] += a;
    state[1] += b;
    state[2] += c;
    state[3] += d;
    state[4] += e;
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  PUBLIC FUNCTIONS
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  Sha1Initialise
//
//  Initialises an SHA1 Context. Use this to initialise/reset a context.
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
void 
    Sha1Initialise
    (
        Sha1Context*                Context
    )
{
    // SHA1 initialization constants
    Context->State[0] = 0x67452301;
    Context->State[1] = 0xEFCDAB89;
    Context->State[2] = 0x98BADCFE;
    Context->State[3] = 0x10325476;
    Context->State[4] = 0xC3D2E1F0;
    Context->Count[0] = 0;
    Context->Count[1] = 0;
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  Sha1Update
//
//  Adds data to the SHA1 context. This will process the data and update the internal state of the context. Keep on
//  calling this function until all the data has been added. Then call Sha1Finalise to calculate the hash.
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
void 
    Sha1Update
    (
        Sha1Context*        Context,
        const void*         Buffer,
        uint32_t            BufferSize
    )
{
    uint32_t    i;
    uint32_t    j;

    j = (Context->Count[0] >> 3) & 63;
    if ((Context->Count[0] += BufferSize << 3) < (BufferSize << 3))
    {
        Context->Count[1]++;
    }

    Context->Count[1] += (BufferSize >> 29);
    if ((j + BufferSize) > 63)
    {
        i = 64 - j;
        memcpy(&Context->Buffer[j], Buffer, i);
        TransformFunction(Context->State, Context->Buffer);
        for (; i + 63 < BufferSize; i += 64)
        {
            TransformFunction(Context->State, (const uint8_t*)Buffer + i);
        }
        j = 0;
    }
    else
    {
        i = 0;
    }

    memcpy(&Context->Buffer[j], &((const uint8_t*)Buffer)[i], BufferSize - i);
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//  Sha1Finalise
//
//  Performs the final calculation of the hash and returns the digest (20 byte buffer containing 160bit hash). After
//  calling this, Sha1Initialised must be used to reuse the context.
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
void 
    Sha1Finalise
    (
        Sha1Context*                Context,
        SHA1_HASH*                  Digest
    )
{
    uint32_t    i;
    uint8_t     finalcount[8];

    for (i = 0; i < 8; i++)
    {
        finalcount[i] = (unsigned char)((Context->Count[(i >= 4 ? 0 : 1)]
         >> ((3-(i & 3)) * 8) ) & 255);  // Endian independent
    }
    Sha1Update(Context, (const uint8_t*)"\x80", 1);
    while ((Context->Count[0] & 504) != 448)
    {
        Sha1Update(Context, (const uint8_t*)"\0", 1);
    }

    Sha1Update(Context, finalcount, 8);  // Should cause a Sha1TransformFunction()
    for (i = 0; i < SHA1_HASH_SIZE; i++)
    {
        Digest->bytes[i] = (uint8_t)((Context->State[i>>2] >> ((3-(i & 3)) * 8) ) & 255);
    }
}
