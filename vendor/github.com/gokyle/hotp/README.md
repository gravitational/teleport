## `hotp`

This package implements the RFC 4226 OATH-HOTP algorithm; these
passwords derived from the HMAC-SHA1 of an internal counter. They
are presented as (typically) 6 or 8-digit numeric passphrases.

This package was designed to be interoperable with the Google
Authenticator app and YubiKeys programmed in OATH-HOTP mode.

Also provided is the `hotpgen` command-line program. This generates
a QR code suitable for use with the Google Authenticator application
alongside a text file containing the URL for the QR code. For more
information, see the README in the file.

See also the [godocs](https://godoc.org/github.com/gokyle/hotp/)
for this package. The [hotpweb](https://github.com/gokyle/hotpweb/)
package provides a simple webapp demonstrating the use of the Google
Authenticator interaction.


### Storing Keys

> **These keys are cryptographic secrets.** Please store them with
> all due caution! For example, they could be encrypted in a database
> using [cryptobox](https://github.com/cryptobox/gocryptobox/).

The HOTP keys can be serialised with the `Marshal` function; this preserves
a "snapshot", so to speak, of the key value. Serialisation is done
in DER-format:

```
SEQUENCE {
	OCTET STRING
	INTEGER
	INTEGER
}
```

Serialised key values can be parsed with the `Unmarshal` function;
as a serialised key value is a snapshot, the counter state at the
time of marshalling will be restored.

If the key values are to be stored in a database, the key and counter
values must be preserved. To avoid any potention issues, the counter
value should be stored using the `Counter` method (i.e., as an
`uint64`) and key values restored with `NewHOTP`. *It is strongly
recommended that the `Key` field be stored securely.* The `Digit`
field can be stored as constant in the program, and used whenever
key values are loaded.


### Example Usages

#### Case 1: Google Authenticator

A server that wants to generate a new HOTP authentication for users
can generate a new random HOTP source; this example saves a QR code
to a file. The user can scan this QR code in with the app on their
phone and use it to generate codes for the server.

	// Generate a new, random 6-digit HOTP source with an initial
	// counter of 0 (the second argument, if true, will randomise
	// the counter).
	otp, err := GenerateHOTP(6, false)
	if err != nil {
		// error handling elided
	}

	qrCode, err := otp.QR("user@example.net")
	if err != nil {
		// error handling elided
	}

	err = ioutil.WriteFile("user@example.net.png", qrCode
	if err != nil {
		// error handling elided
	}

After the user has imported this QR code, they can immediately begin
using codes for it. The `Check` method on an OTP source will check
whether the code is valid; if it isn't, the counter won't be
decremented to prevent the server from falling out of sync. If the
either side is suspected of falling out of sync, the `Scan` method
will look ahead a certain window of values. If it finds a valid
value, the counter is updated and the two will be in sync again.

The Google Authenticator app on Android also provides users a means
to "Check key value"; the `IntegrityCheck` method will provide the
the two values shown here (the initial code and the current counter)
that may be used to verify the integrity of the key value.

In the `testdata` directory, there are three files with the base name
of "gauth_example" that contain the HOTP key values used in the
test suite. The PNG image may be scanned in using a mobile phone,
the text file contains the URL that the QR code is based on, and
the `.key` file may be used with the `hotpcli` program. The first
several codes produced by this URL are:

* 023667, counter = 0
* 641344, counter = 1
* 419615, counter = 2
* 692589, counter = 3
* 237233, counter = 4
* 711695, counter = 5
* 620195, counter = 6

The codes may be checked against the app to ensure they are correct;
these values are used in the test suite to ensure interoperability.


#### Case 2: YubiKey

A YubiKey programmed in "OATH-HOTP" mode can also be used with this
package. The YubiKey user will need to provide their key, and
optionally their token identifier for additional security. If the
token is used across multiple sites, the `Scan` function will need
to be used (with a probably generous window) to sync the counters
initially.

When reading input from a YubiKey, the `YubiKey` method takes as
input the output directly from the token, and splits it into the
code, the token identity, and a boolean indicating whether it was
valid output. Note that `YubiKey` **does not** check whether the
code is correct; this ensures that the user can check the code with
whatever means is appropriate (i.e. `Scan` or `Check`).

In the `testdata` directory, there is a configuration file containing
the paramters for the YubiKey used to generate the test suite. This
may be used to program a YubiKey to verify the package's interoperability.
The first several codes produced by this configuration are (split
into raw output from yubikey / the code / the counter):

* cccc52345777705179, 705179, counter = 0
* cccc52345777404068, 404068, counter = 1
* cccc52345777490143, 490143, counter = 2
* cccc52345777739740, 739740, counter = 3
* cccc52345777043269, 043269, counter = 4
* cccc52345777035666, 035666, counter = 5
* cccc52345777725326, 725325, counter = 6


### TODO

* Add example code.


### Test Coverage

Test coverage is currently at 100%.

#### Current test status

[![Build Status](https://drone.io/github.com/gokyle/hotp/status.png)](https://drone.io/github.com/gokyle/hotp/latest)


### References

* [RFC 4226 - *HOTP: An HMAC-Based One-Time Password Algorithm*](http://www.ietf.org/rfc/rfc4226.txt)
is the specification of the OATH-HOTP algorithm. A copy is provided
in the package's source directory.

* The [Key URI Format](https://code.google.com/p/google-authenticator/wiki/KeyUriFormat)
page on the Google Authenticator wiki documents the URI format for
use with the Google Authenticator app. This package follows that
format when generating URLs (and by extension, QR codes).

* The [YubiKey manual](http://www.yubico.com/wp-content/uploads/2013/07/YubiKey-Manual-v3_1.pdf)
contains documentation on the YubiKey HOTP format.


### Author

`hotp` was written by Kyle Isom <kyle@tyrfingr.is>.


### License

```
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
```
