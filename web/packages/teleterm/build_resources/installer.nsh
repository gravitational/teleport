# https://github.com/electron-userland/electron-builder/blob/v24.0.0-alpha.5/docs/configuration/nsis.md#custom-nsis-script

# electron-builder adds `BUILD_RESOURCES_DIR\x86-unicode` as a plugin dir.
# But that dir name isn't very descriptive, so we add a custom plugin dir.
!addplugindir "${BUILD_RESOURCES_DIR}\nsis-plugins"

# The EnVar plugin is recommended for env var modification as EnvVarUpdate doesn't handle long
# strings very well.
# https://nsis.sourceforge.io/Environmental_Variables:_append,_prepend,_and_remove_entries
# https://nsis.sourceforge.io/EnVar_plug-in

!macro customInstall
    # Make EnVar define system env vars since Connect is installed per-machine.
    EnVar::SetHKLM
    EnVar::AddValue "Path" $INSTDIR\resources\bin

    nsExec::ExecToStack '"$INSTDIR\resources\bin\tsh.exe" vnet-install-service'
    Pop $0 # ExitCode
    Pop $1 # Output
    ${If} $0 != 0
        MessageBox MB_ICONSTOP \
            "tsh.exe vnet-install-service failed with exit code $0. Output: $1"
        Quit
    ${Endif}
!macroend

!macro customUnInstall
    EnVar::SetHKLM
    # Inside the uninstaller, $INSTDIR is the directory where the uninstaller lies.
    # Fortunately, electron-builder puts the uninstaller directly into the actual installation dir.
    # https://nsis.sourceforge.io/Docs/Chapter4.html#varother
    EnVar::DeleteValue "Path" $INSTDIR\resources\bin

    nsExec::ExecToStack '"$INSTDIR\resources\bin\tsh.exe" vnet-uninstall-service'
    Pop $0 # ExitCode
    Pop $1 # Output
    ${If} $0 != 0
        MessageBox MB_ICONSTOP \
            "tsh.exe vnet-uninstall-service failed with exit code $0. The uninstaller is going to continue. Output: $1"
    ${Endif}
!macroend
