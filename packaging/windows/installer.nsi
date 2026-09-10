Unicode True
!include "MUI2.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!ifndef PAYLOAD
  !error "PAYLOAD is required"
!endif
!ifndef OUTPUT
  !error "OUTPUT is required"
!endif
Name "El Ciber · Alpha"
OutFile "${OUTPUT}"
InstallDir "$PROGRAMFILES64\El Ciber"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
BrandingText "El Ciber · Vista previa técnica"
ShowInstDetails show
ShowUninstDetails show
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TITLE "Instalar El Ciber"
!define MUI_WELCOMEPAGE_TEXT "El instalador prepara el cliente y descarga automáticamente su motor de red. No necesitas instalar EasyTier, Go o Python por separado.$\r$\n$\r$\nNecesita Internet y permisos de Windows. Esta alpha todavía no está lista para jugar entre casas: falta activar el servicio de encuentro y validar partidas reales.$\r$\n$\r$\nNo desactiva el firewall ni instala un servicio de arranque."
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${PAYLOAD}\LICENSE.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_TEXT "El Ciber está instalado. Ábrelo desde el menú Inicio.$\r$\n$\r$\nWindows pedirá permiso para el adaptador al iniciar el cliente. La alpha aún necesita su servicio de encuentro y pruebas entre equipos; no es una versión lista para jugar.$\r$\n$\r$\nCerrar la pestaña no detiene el cliente: utiliza Cerrar El Ciber dentro de la aplicación."
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "Spanish"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "Esta versión requiere Windows de 64 bits."
    Abort
  ${EndIf}
  ${IfNot} ${AtLeastWin10}
    MessageBox MB_ICONSTOP "Esta versión requiere Windows 10 u 11."
    Abort
  ${EndIf}
FunctionEnd

Section "El Ciber" SEC_MAIN
  SetRegView 64
  SetShellVarContext all
  IfFileExists "$INSTDIR\elciber.exe" existing
  IfFileExists "$INSTDIR\engines\easytier\*" existing
  Goto prepare
existing:
  MessageBox MB_ICONSTOP "Ya hay una instalación en esta carpeta. Cierra y desinstala la anterior antes de continuar. Las salas del perfil se conservan. Esta alpha no actualiza instalaciones en uso."
  Abort
prepare:
  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File "${PAYLOAD}\setup-engine.exe"
  DetailPrint "Descargando y verificando el motor oficial..."
  ExecWait '"$PLUGINSDIR\setup-engine.exe" --destination "$INSTDIR\engines\easytier" --platform windows' $0
  ${If} $0 != 0
    MessageBox MB_ICONSTOP "No se pudo preparar el motor. No se ha instalado el cliente. Comprueba la conexión a Internet, el espacio y los permisos. Nunca desactives la protección del equipo para continuar."
    Abort
  ${EndIf}
  SetOutPath "$INSTDIR"
  File "${PAYLOAD}\elciber.exe"
  File "${PAYLOAD}\LICENSE.txt"
  File "${PAYLOAD}\THIRD-PARTY.txt"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateShortcut "$SMPROGRAMS\El Ciber.lnk" "$INSTDIR\elciber.exe" "--enable-vpn"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber" "DisplayName" "El Ciber · Alpha"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber" "DisplayVersion" "0.1.0-alpha.1"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber" "Publisher" "El Ciber contributors"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber" "UninstallString" '$\"$INSTDIR\uninstall.exe$\"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber" "URLInfoAbout" "https://github.com/EspacioKoop/elciber"
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber" "NoRepair" 1
SectionEnd

Section "Uninstall"
  SetRegView 64
  SetShellVarContext all
  MessageBox MB_OKCANCEL "Cierra El Ciber antes de continuar. Se retiran sus archivos instalados; las salas y secretos de tu perfil NO se borran." IDOK +2
  Abort
  ClearErrors
  Delete "$INSTDIR\elciber.exe"
  IfErrors locked
  Delete "$INSTDIR\engines\easytier\easytier-core.exe"
  IfErrors locked
  Delete "$INSTDIR\engines\easytier\easytier-cli.exe"
  Delete "$INSTDIR\engines\easytier\wintun.dll"
  Delete "$INSTDIR\engines\easytier\manifest.json"
  RMDir "$INSTDIR\engines\easytier"
  RMDir "$INSTDIR\engines"
  Delete "$INSTDIR\LICENSE.txt"
  Delete "$INSTDIR\THIRD-PARTY.txt"
  Delete "$SMPROGRAMS\El Ciber.lnk"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\ElCiber"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Goto done
locked:
  MessageBox MB_ICONSTOP "Hay archivos en uso. Cierra El Ciber y vuelve a ejecutar la desinstalación."
  Abort
done:
SectionEnd
