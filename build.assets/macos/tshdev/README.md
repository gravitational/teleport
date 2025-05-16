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
codesign -dv --verbose=4                    # verifies code signatures

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

## FIDO2

In order to have a signed binary with FIDO2 support, you need to statically link libfido2.

```
make build-fido2
PKG_CONFIG_PATH=$(make print-fido2-pkg-path) FIDO2=static make build/tsh
```

## Working with launch daemons

tsh.app includes a launch daemon for VNet under `Contents/Library/LaunchDaemons`. tsh uses
[`SMAppService`](https://developer.apple.com/documentation/servicemanagement/smappservice?language=objc)
to register the daemon plist. Once the daemon is registered and is enabled under login items, it can
be started by sending a message to the XPC service advertised under `MachServices` in plist. This is
going to make launchd start the program listed under `BundleProgram`. Extra arguments are passed to
the program are under `ProgramArguments`. See `man launchd.plist` for more details behind individual
plist properties.

When there's no login item for a daemon and an app attempts to register it for the first time, macOS
shows a notification about it which says "tsh.app added items that can run in the background for all
users. Do you want to allow this?". When working on a launch daemon, at some point macOS is going to
stop showing the notification. In Console.app, you're going to see this:

```
Exceeded max notifications for tsh. Please file a Feedback report or contact the developer.
item=uuid=4DF802A6-A9DE-4EF8-A6FB-4B046E50E1DE, name=tsh, type=daemon, disposition=[enabled,
disallowed, visible, not notified], identifier=com.goteleport.tshdev.vnetd,
url=Contents/Library/LaunchDaemons/com.goteleport.tshdev.vnetd.plist -- file:///
```

### Recompiling daemon

Once a login item is enabled, you are free to update the binary advertised under `BundleProgram` as
much as you want. Just remember to sign it before calling the daemon.

### Modifying plist

The situation is more complicated for modifying the plist itself. In theory, macOS should pick up on
any updates made to the plist. In practice, the mechanism for that seems to be spotty, especially on
development machines where apps are being rebuilt many times:

* [Unable to register loginItem via SMAppService - Status Error 78](https://developer.apple.com/forums/thread/726826)
* [Login Item failing to launch with SMAppService. Error: 78, LastExitStatus: 19968](https://forums.developer.apple.com/forums/thread/727785)

When working on a prototype of the launch daemon, we've ran into a situation where macOS
aggressively cached information from plists. Despite `BundleProgram` being changed to another path,
macOS did not picked up on that.

To see how macOS interprets the plist, there are two helpful commands:

To dump information from the database of Launch Services (search for the identifier of the bundle or
the label of the XPC service):

```
sfltool dumpbtm
```

To see how launchd interprets the plist:

```
launchctl print system/com.goteleport.tshdev.vnetd
```

If you find that macOS doesn't have up to date info about the plist, then…

### Refreshing cached plist information

The method we had the most success with involves these steps:

1. If you made any changes inside `tsh.app`, like modifying plists, make sure to stage those changes
   in git first.
1. Close system settings if they're open.
1. Move `tsh.app` to the trash and then either empty the trash or delete that specific item from the
   trash.
1. Open Login Items in system settings.

This forces the Launch Services database to get updated. After opening Login Items, you might be
able to see the old login item for tsh.app for a split second after which it's removed from the
list.

This completely resets the state of the login item, meaning that the daemon will have to be
registered and enabled again. Now restore deleted files from tsh.app in git and then rebuild and
sign the app. Hopefully, this time macOS will pick up any new changes.

If that doesn't work, as a last resort there's a **very destructive** `sfltool resetbtm`. This
basically wipes the whole database of Launch Services, meaning that the status of all login items
gets reset – this includes other software installed on your machine.

After running it, you must restart the device. We found it to be the most helpful when switching
between local signed builds and tag builds. For some reason, in that scenario macOS wouldn't want to
launch the daemon with the following error:

```
2024-07-03 17:24:13.522640 (system/com.goteleport.tshdev.vnetd [70808]) <Warning>: Could not find and/or execute program specified by service: 3: No such process: Contents/MacOS/tsh
2024-07-03 17:24:13.522662 (system/com.goteleport.tshdev.vnetd [70808]) <Error>: Service could not initialize: copy_bundle_path(<some binary data here>, ?pn-, 0, 0), error 0x6f - Invalid or missing Program/ProgramArguments
```

After resetting the db and restarting the device, everything seemed to be working again.

In theory, it's possible to list all app bundles with a certain bundle identifier by running the
following command:

```
mdfind kMDItemCFBundleIdentifier = "com.goteleport.tshdev"
```

In practice, getting rid of all but one bundle didn't appear to solve the problem.

### Daemon does not start

List all jobs loaded into launchd. The second column is the status which you can then inspect.

```
$ sudo launchctl list | grep vnetd
-   78   com.goteleport.tshdev.vnetd
$ launchctl error 78
78: Function not implemented
```

In that scenario, the `launchctl print` command above might return information that's of more use.
Ultimately, you want to look at the logs from launchd – they seem to be the most helpful when
debugging issues.

```
tail -f /var/log/com.apple.xpc.launchd/launchd.log
```

Capturing logs in Console.app might be useful too. However, the logs from launchd were sufficient
for any debugging we had to do so far.
