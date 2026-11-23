; Docklet installer (Inno Setup 6.3 or later). Built by `wails3 task windows:create:inno:installer`:
;   ISCC /DAppVersion=0.6.3 /DArch=amd64 /DSourceExe=<path to docklet.exe> /DOutputDir=<dir> docklet.iss
; Arch is amd64 or arm64. The setup program itself is 32-bit x86 (it also runs on Windows on ARM)
; and installs the native binary of its architecture.

#ifndef AppVersion
  #error AppVersion is required, e.g. /DAppVersion=0.6.3
#endif
#ifndef Arch
  #define Arch "amd64"
#endif
#ifndef SourceExe
  #error SourceExe is required: the path of the docklet.exe to install
#endif
#ifndef OutputDir
  #define OutputDir "..\..\..\bin"
#endif

; Windows file versions are numeric: 0.6.3-rc.1 is stamped as 0.6.3.
#define NumericVersion (Pos("-", AppVersion) > 0 ? Copy(AppVersion, 1, Pos("-", AppVersion) - 1) : AppVersion)
#define AppName "Docklet"
#define AppExe "docklet.exe"
#define Publisher "Docklet"
; The key the earlier NSIS installer wrote; an upgrade takes over its folder and removes its entry.
#define NsisUninstallKey "Software\Microsoft\Windows\CurrentVersion\Uninstall\DockletDocklet"

[Setup]
; Never change AppId: Windows identifies the installed app by it for upgrades and uninstall.
AppId={{6F4B1E2A-8C3D-4F5E-9A7B-D0C1E2F3A4B5}
AppName={#AppName}
AppVersion={#AppVersion}
AppVerName={#AppName} {#AppVersion}
AppPublisher={#Publisher}
AppPublisherURL=https://github.com/koderi-dp/docklet
AppSupportURL=https://github.com/koderi-dp/docklet/issues
AppUpdatesURL=https://github.com/koderi-dp/docklet/releases
AppCopyright=(c) 2026 Danijel
VersionInfoVersion={#NumericVersion}
VersionInfoProductVersion={#NumericVersion}
VersionInfoProductTextVersion={#AppVersion}
VersionInfoDescription={#AppName} Installer
DefaultDirName={code:DefaultInstallDir}
DisableProgramGroupPage=yes
PrivilegesRequired=admin
#if Arch == "arm64"
ArchitecturesAllowed=arm64
ArchitecturesInstallIn64BitMode=arm64
#else
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
#endif
; Docklet's single-instance mutex (Wails: "wails-app-" + UniqueID + "-sim"); setup asks to close it.
AppMutex=wails-app-dev.docklet.app-sim
CloseApplications=yes
RestartApplications=no
OutputDir={#OutputDir}
OutputBaseFilename=docklet-{#Arch}-installer
SetupIconFile=..\icon.ico
UninstallDisplayIcon={app}\{#AppExe}
UninstallDisplayName={#AppName}
WizardStyle=modern
Compression=lzma2/max
SolidCompression=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "{#AppExe}"; Flags: ignoreversion
Source: "..\nsis\MicrosoftEdgeWebview2Setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall; Check: NeedsWebView2

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\{#AppExe}"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\{#AppExe}"; Tasks: desktopicon

[Run]
Filename: "{tmp}\MicrosoftEdgeWebview2Setup.exe"; Parameters: "/silent /install"; StatusMsg: "Installing the WebView2 Runtime..."; Flags: waituntilterminated; Check: NeedsWebView2
Filename: "{app}\{#AppExe}"; Description: "{cm:LaunchProgram,{#AppName}}"; Flags: nowait postinstall skipifsilent

[Code]
const
  WebView2Key = 'Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';

function NeedsWebView2: Boolean;
var
  Version: String;
begin
  Result := not (
    (RegQueryStringValue(HKLM, 'SOFTWARE\WOW6432Node\' + WebView2Key, 'pv', Version) and (Version <> '') and (Version <> '0.0.0.0')) or
    (RegQueryStringValue(HKCU, 'Software\' + WebView2Key, 'pv', Version) and (Version <> '') and (Version <> '0.0.0.0')));
end;

{ The folder of an earlier NSIS install, taken from its uninstall entry; '' when there is none. }
function NsisInstallDir: String;
var
  Uninstall: String;
begin
  Result := '';
  if RegQueryStringValue(HKLM64, '{#NsisUninstallKey}', 'UninstallString', Uninstall) then
    Result := ExtractFileDir(RemoveQuotes(Uninstall));
end;

function DefaultInstallDir(Param: String): String;
begin
  Result := NsisInstallDir;
  if Result = '' then
    Result := ExpandConstant('{autopf}\{#Publisher}\{#AppName}');
end;

{ After an upgrade from the NSIS installer: drop its uninstaller and its Apps entry, so Docklet is
  listed once. Its shortcuts have the same paths as ours and were replaced. User data is untouched. }
procedure CurStepChanged(CurStep: TSetupStep);
var
  OldDir: String;
begin
  if CurStep <> ssPostInstall then
    Exit;
  OldDir := NsisInstallDir;
  if OldDir = '' then
    Exit;
  DeleteFile(AddBackslash(OldDir) + 'uninstall.exe');
  RegDeleteKeyIncludingSubkeys(HKLM64, '{#NsisUninstallKey}');
end;
