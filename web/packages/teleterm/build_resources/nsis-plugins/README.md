By default, electron-builder adds `${buildResources}\x86-unicode` as the plugin dir. But that name
is not really that descriptive, so we put the plugins under nsis-plugins.

When you download a plugin, its `Plugins` folder is likely to contain .dlls for different
architectures and encodings such as amd64-unicode, x86-ansi, x86-unicode. You should use the .dlls
from the `x86-unicode` dir.
