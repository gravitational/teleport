# tshdev

The `tshdev` app skeleton is used to test features that require a
signed/entitled `tsh` binary in development environments (eg, touch ID).

## Instructions

```shell
skel=build.assets/macos/tshdev
go build -tags=touchid ./tool/tsh            # at Teleport root
mkdir -p $skel/tsh.app/Contents/MacOS/
cp tsh $skel/tsh.app/Contents/MacOS/         # copy binary to .app
$skel/sign.sh $skel/tsh.app                  # sign .app
$skel/tsh.app/Contents/MacOS/tsh touchid ls  # use tsh
```

Alternatively, you may install `tshdev.provisionprofile` locally, then sign and
run the "naked" binary. To install the profile, open it using Finder.

```shell
skel=build.assets/macos/tshdev
go build -tags=touchid ./tool/tsh  # at Teleport root
$skel/sign.sh tsh                  # sign tsh binary
./tsh touchid ls                   # use tsh
```

## Structure

* `tsh.app`                 - macOS .app skeleton
* `tshdev.entitlements`     - entitlements claimed by tsh the binary
* `tshdev.provisionprofile` - provisioning profile (also embedded in .app)

## Useful commands

```shell
security find-identity -vp codesigning      # list codesign identities
security cms -D -i /path/to/profile         # inspect provisioning profile
codesign -d --entitlements /path/to/binary  # shows binary entitlements

# Extract plist and certificate from profile
security cms -D -i /path/to/my.provisionprofile -o my.plist
/usr/libexec/PlistBuddy -c 'Print :DeveloperCertificates:0' my.plist > my0.cer
certtool d tshdev0.cer
```

## One-time setup

This section explains how the tshdev skeleton was built. You don't have to go
through the steps here unless you are trying to recreate parts of the skeleton
(or trying to create a new skeleton).

1. Create a provisioning profile on
   https://developer.apple.com/account/resources/profiles/list

    In order to create the profile, a Developer ID certificate and a Developer
    ID app are necesary.

    For `tshdev` we use:

    - Team `K497G57PDJ` / apple-developer@goteleport.com
    - App `com.goteleport.tshdev`
    - Developer ID certificate `A5604F285B0957134EA099AC515BD9E0787228AC`
    - See https://developer.apple.com/account/resources/profiles/review/JGACZBU48V

    Make sure the profile contains the necessary entitlements and that the .app
    claims those entitlements as well.

2. Install the developer certificates via XCode
3. Prepare a daemon-in-app skeleton

    See
    https://developer.apple.com/documentation/xcode/signing-a-daemon-with-a-restricted-entitlement.

    The app and bundle must match the provisioning profile above. Use the
    provisioning profile downloaded from Apple instead of letting XCode manage
    it.

4. Export the app using XCode

    For example, click "build" and copy the app using "Product -> Show Build
    Folder in Finder".
