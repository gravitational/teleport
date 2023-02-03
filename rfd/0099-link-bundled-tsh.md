---
authors: Rafał Cieślak (rafal.cieslak@goteleport.com)
state: implemented
---

# RFD 99 - Making bundled tsh available outside of Connect

## Required Approvers

* Engineering: @avatus @gzdunek

## What

Make bundled tsh available to use outside of Connect by putting the binary on user's `PATH`.

## Why

As of now, Connect ships with a bundled version of tsh. When a new local shell is started from
within Connect, the directory with that bundled tsh is added to `PATH`. This makes tsh available
within that local shell, even if the user doesn't have tsh available in their usual `PATH`.

Recently, Connect became the default download option for macOS and Windows. People who install
Connect can use tsh from within Connect. But if they install just Connect, tsh will not be available
from any other terminal emulator.

We want to permanently put the bundled tsh in `PATH` so that it's available to use outside of
Connect.

## Details

Before we talk about how Connect can do it, let's look at how VSCode and electron-builder accomplish
this task.

### How VSCode does it

We can take a lot of hints from VSCode because it's open-source and also distributes a binary called
[`code`](https://code.visualstudio.com/docs/editor/command-line).

macOS is the only platform where making `code` available requires user interaction. That is because
VSCode is distributed as .app and doesn't require a separate installation step where the user could
be asked to grant the necessary permissions to the installer.

#### Linux

`code` is automatically linked to `/usr/bin` during installation. Notably, VSCode uses [a SPEC
file](https://rpm-packaging-guide.github.io/#what-is-a-spec-file) for its rpm package. As shown by
[microsoft/vscode#142907](https://github.com/microsoft/vscode/pull/142907), this lets rpm correctly
recognize that the `code` binary is owned by the VSCode package. This also means that rpm will
automatically clean up this file when the user uninstalls the package.

The deb package accomplishes the same goal through post install and post removal scripts.

* https://github.com/microsoft/vscode/blob/1.74.0/resources/linux/debian/postinst.template#L6-L8
* https://github.com/microsoft/vscode/blob/1.74.0/resources/linux/debian/postrm.template#L6
* https://github.com/microsoft/vscode/blob/1.74.0/resources/linux/rpm/code.spec.template#L34

#### Windows

During installation, the directory with `code` is added to the `Path` env variable.

* https://github.com/microsoft/vscode/blob/1.74.0/build/win32/code.iss#L1299

#### macOS

The command palette offers [separate
commands](https://code.visualstudio.com/docs/setup/mac#_launching-from-the-command-line) for
installing and uninstalling the CLI tool. The installation command executes an AppleScript with
administrator privileges which symlinks `code` to `/usr/local/bin/code`.

* https://github.com/microsoft/vscode/blob/1.74.0/src/vs/workbench/electron-sandbox/actions/installActions.ts
* https://github.com/microsoft/vscode/blob/1.74.0/src/vs/platform/native/electron-main/nativeHostMainService.ts#L261

### How electron-builder does it

#### Linux

Linux is the only platform where during the installation of a package, electron-builder
automatically symlinks the binary to `/usr/bin`.

Unlike VSCode, electron-builder doesn't have special templates for any particular package format.
Instead it uses [fpm](https://fpm.readthedocs.io) which handles many different formats.

The binary is linked through a post install and post removal scripts. This means that rpm doesn't
recognize that the binary comes from the specific package.

* https://github.com/electron-userland/electron-builder/blob/v24.0.0-alpha.5/packages/app-builder-lib/templates/linux/after-install.tpl
* https://github.com/electron-userland/electron-builder/blob/v24.0.0-alpha.5/packages/app-builder-lib/templates/linux/after-remove.tpl

### How Connect can do it

#### Linux

We can automatically link tsh using the same method that electron-builder uses. Unfortunately,
electron-builder doesn't seem to provide a way to extend the post install script in any way. This
most likely means that we'll have to copy the default script and keep it in sync when updating
electron-builder.

Instead of symlinking the binary to `/usr/bin/tsh`, we can symlink it to `/usr/local/bin/tsh`. This
is where our Linux teleport package places its binaries. We should assume that if both teleport and
teleport-connect packages are installed, the user prefers the tsh version from the teleport package.

* Connect should create the symlink only if it doesn't exist already.
* Connect should remove the symlink only if it points at the tsh bundled with Connect.
* If Connect decides not to create or remove the symlink, it should echo a message explaining the
  reason.

This ensures that the teleport and teleport-connect packages don't fight with each other.

When adding the custom post install script, we might want to change the location of the
`teleport-connect` symlink to `/usr/local/bin` as well.

#### Windows

We use NSIS to build the Windows installer. electron-builder offers a way to extend the scripts
executed by the installer by [defining custom
macros](https://www.electron.build/configuration/nsis#custom-nsis-script). NSIS has [built-in
utilities to manipulate the `Path` env var](https://nsis.sourceforge.io/Path_Manipulation), however
at the top of that page there's a big red banner telling you to use [the EnVar
plugin](https://nsis.sourceforge.io/EnVar_plug-in) for path manipulation instead due to length
limitations in the built-in utilities.

We will have to figure out how to use that plugin but once that's done, Connect should simply add
the bin folder to the path without creating symlinks of any kind, just as VSCode does.

#### macOS

For now we've decided to introduce a custom `tsh install` command to the suggested commands in the
command bar. This command won't be a real tsh command. Instead it'll execute an AppleScript, similar
to the VSCode command.

Similar to Linux, we can symlink the binary to `/usr/local/bin/tsh`. The tsh .pkg installer also
places the binary there. We'll still need to ask about administrator privileges because that
directory is owned by root.

Running the command will overwrite an existing symlink if present.

We will also add `tsh uninstall` which will remove the symlink. Both commands will be present only
in the macOS version of the app.
