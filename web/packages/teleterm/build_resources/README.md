This is the directory we use as the `buildResources` dir for electron-builder.

By default, electron-builder uses the `build` dir at the project root. However, we already use that
directory for the build output from Vite.

If you see a path in electron-builder docs referring to `build`, you can assume that they meant the
`buildResources` directory.
