/*
Package hotp implements the RFC 4226 OATH-HOTP algorithm; these
passwords derived from the HMAC-SHA1 of an internal counter. They
are presented as (typically) 6 or 8-digit numeric passphrases.

The package provides facilities for interacting with YubiKeys
programmed in OATH-HOTP mode, as well as with the Google
Authenticator application. The package also provides QR-code
generation for new OTPs.
*/
package hotp

/*
   Copyright (c) 2013 Kyle Isom <kyle@tyrfingr.is>

   Permission to use, copy, modify, and distribute this software for any
   purpose with or without fee is hereby granted, provided that the above
   copyright notice and this permission notice appear in all copies.

   THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
   WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
   MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
   ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
   WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
   ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
   OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
*/
