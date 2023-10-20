# https://github.com/electron-userland/electron-builder/blob/v24.0.0-alpha.5/docs/configuration/nsis.md#custom-nsis-script

# electron-builder adds `BUILD_RESOURCES_DIR\x86-unicode` as a plugin dir.
# But that dir name isn't very descriptive, so we add a custom plugin dir.
!addplugindir "${BUILD_RESOURCES_DIR}\nsis-plugins"

# The EnVar plugin is recommended for env var modification as EnvVarUpdate doesn't handle long
# strings very well.
# https://nsis.sourceforge.io/Environmental_Variables:_append,_prepend,_and_remove_entries
# https://nsis.sourceforge.io/EnVar_plug-in

!macro customInstall
    # Make EnVar define user env vars instead of system env vars.
    EnVar::SetHKCU
    EnVar::AddValue "Path" $INSTDIR\resources\bin
!macroend

!macro customUnInstall
    EnVar::SetHKCU
    # Inside the uninstaller, $INSTDIR is the directory where the uninstaller lies.
    # Fortunately, electron-builder puts the uninstaller directly into the actual installation dir.
    # https://nsis.sourceforge.io/Docs/Chapter4.html#varother
    EnVar::DeleteValue "Path" $INSTDIR\resources\bin
!macroend
