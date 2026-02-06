# https://github.com/electron-userland/electron-builder/blob/v24.0.0-alpha.5/docs/configuration/nsis.md#custom-nsis-script

# electron-builder adds `BUILD_RESOURCES_DIR\x86-unicode` as a plugin dir.
# But that dir name isn't very descriptive, so we add a custom plugin dir.
!addplugindir "${BUILD_RESOURCES_DIR}\nsis-plugins"

# The EnVar plugin is recommended for env var modification as EnvVarUpdate doesn't handle long
# strings very well.
# https://nsis.sourceforge.io/Environmental_Variables:_append,_prepend,_and_remove_entries
# https://nsis.sourceforge.io/EnVar_plug-in

# To inform the user that VNet is available only in the per-machine mode, we need to display a message in the wizard.
# It could be added on a separate welcome page (which can be fully customized), but that would introduce an
# additional step in the wizard and create unnecessary friction.
# Instead, we modify the existing "selectUserMode" and "forAll" strings by hand (electron-builder doesn't allow customizing the translations from
# https://github.com/electron-userland/electron-builder/blob/6c20eeb1cf9fd10980cde3c9ce0602fa6b7c6972/packages/app-builder-lib/templates/nsis/assistedMessages.yml).
# Important: the message can't be too long, the template was designed for around two lines of text, the rest is clipped.
#
# Because of electron-builder's default setting which treats warnings as errors, we need to disable warnings 6030
# (warning 6030: LangString "selectUserMode" set multiple times for 1033, wasting space).

!pragma warning disable 6030
!macro customHeader
  LangString selectUserMode ${LANG_ENGLISH} "Select installation mode. Only the 'Anyone who uses this computer' option comes with VNet, Teleport's VPN-like experience for accessing TCP applications and SSH servers."
  LangString forAll ${LANG_ENGLISH} "Anyone who uses this computer (&all users). Includes VNet support."
!macroend

!macro customInstall
  ${If} $installMode == "all"
    # Make EnVar define system env vars when the app is installed per-machine.
    EnVar::SetHKLM
    EnVar::AddValue "Path" $INSTDIR\resources\bin

    nsExec::ExecToStack '"$INSTDIR\resources\bin\tsh.exe" vnet-install-service'
    Pop $0 # ExitCode
    Pop $1 # Output
    ${If} $0 != 0
        MessageBox MB_ICONSTOP \
            "tsh.exe vnet-install-service failed with exit code $0. The installer is going to continue. Output: $1"
    ${Endif}

    nsExec::ExecToStack '"$INSTDIR\resources\bin\tsh.exe" connect-updater-install-service'
    Pop $0 # ExitCode
    Pop $1 # Output
    ${If} $0 != 0
        MessageBox MB_ICONSTOP \
            "tsh.exe connect-updater-install-service failed with exit code $0. The installer is going to continue. Output: $1"
    ${Endif}

  ${Else}
    # Make EnVar define system user vars when the app is installed per-user.
    EnVar::SetHKCU
    EnVar::AddValue "Path" "$INSTDIR\resources\bin"

  ${EndIf}
!macroend

!macro customUnInstall
  ${If} $installMode == "all"
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

    nsExec::ExecToStack '"$INSTDIR\resources\bin\tsh.exe" connect-updater-uninstall-service'
    Pop $0 # ExitCode
    Pop $1 # Output
    ${If} $0 != 0
        MessageBox MB_ICONSTOP \
            "tsh.exe connect-updater-uninstall-service failed with exit code $0. The installer is going to continue. Output: $1"
    ${Endif}

  ${Else}
    EnVar::SetHKCU
    EnVar::DeleteValue "Path" "$INSTDIR\resources\bin"
  ${EndIf}
!macroend
