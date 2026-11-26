$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $PSScriptRoot
Set-Location $RootDir

$env:PATH = "C:\Program Files\Go\bin;C:\Program Files\nodejs;$env:USERPROFILE\go\bin;" + $env:PATH
$env:CGO_ENABLED = "0"

Write-Host "==> [1/6] Ejecutando suite completa de tests Go..." -ForegroundColor Cyan
go test -count=1 ./...
if ($LASTEXITCODE -ne 0) { throw "go test fallo con codigo $LASTEXITCODE" }

Write-Host "==> [2/6] Compilando frontend React + TypeScript..." -ForegroundColor Cyan
Push-Location frontend
try {
    npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw "npm.cmd run build fallo con codigo $LASTEXITCODE" }
} finally {
    Pop-Location
}

Write-Host "==> [3/6] Compilando binario nativo NetPulse.exe (CGO=0, sin UPX)..." -ForegroundColor Cyan
wails build -clean -s -ldflags "-s -w"
if ($LASTEXITCODE -ne 0) { throw "wails build fallo con codigo $LASTEXITCODE" }

$ExePath = Join-Path $RootDir "build\bin\NetPulse.exe"
if (-not (Test-Path $ExePath)) { throw "No se encontro $ExePath" }

$ReleaseDir = Join-Path $RootDir "release"
New-Item -ItemType Directory -Force -Path $ReleaseDir | Out-Null

$Version = "1.2.0"

$ReadmeText = @"
NetPulse v$Version — Portable Windows x64
======================================

100% Local-First · 0% Telemetría · Modo Discord integrado.

USO RÁPIDO:
- Ejecutá NetPulse.exe directamente (no requiere Python, Node ni Npcap).
- Modo Discord viene ACTIVADO por defecto: oculta IPs, MACs, BSSIDs, SSIDs y rutas de usuario.
- Incluye: Instancia única Win32, Inicio con Windows opcional, ES/EN + Tema Oscuro/Claro,
  Atajo Ctrl+Shift+D (Privacidad) y F5 (Actualizar), MTR continuo, Auditoría CGNAT/MTU/IPv6,
  Histórico 30d con exportación CSV/HTML ISP, Monitor de conexiones por proceso sin drivers,
  Perfiles de red Radar LAN con alerta de MAC Gateway, Wi-Fi 6 GHz, SSH TOFU y Bandeja Windows.

AVISOS DE WINDOWS 11:
1. SmartScreen (binario comunitario sin firma comercial):
   Si Windows muestra "Windows protegió su PC", hacé clic en "Más información" -> "Ejecutar de todas formas".
2. Espectro Wi-Fi en Windows 11 24H2+:
   Si aparece el aviso de Ubicación, activá "Permitir que las aplicaciones de escritorio accedan a tu ubicación"
   en Configuración -> Privacidad y seguridad -> Ubicación. Ningún dato sale de tu PC.

VERIFICAR INTEGRIDAD (PowerShell):
   Get-FileHash .\NetPulse.exe -Algorithm SHA256
   Compará el resultado con SHA256SUMS.txt.
"@

$ReadmePath = Join-Path $ReleaseDir "LEEME.txt"
Set-Content -Path $ReadmePath -Value $ReadmeText -Encoding UTF8

Write-Host "==> [4/6] Empaquetando ZIP portable..." -ForegroundColor Cyan
$StageDir = Join-Path $ReleaseDir "_stage_portable"
if (Test-Path $StageDir) { Remove-Item -Recurse -Force $StageDir }
New-Item -ItemType Directory -Force -Path $StageDir | Out-Null
Copy-Item $ExePath (Join-Path $StageDir "NetPulse.exe")
Copy-Item $ReadmePath (Join-Path $StageDir "LEEME.txt")

$ZipName = "NetPulse-v$Version-win64-portable.zip"
$ZipPath = Join-Path $ReleaseDir $ZipName
if (Test-Path $ZipPath) { Remove-Item -Force $ZipPath }
Compress-Archive -Path (Join-Path $StageDir "*") -DestinationPath $ZipPath -CompressionLevel Optimal -Force
Remove-Item -Recurse -Force $StageDir

Write-Host "==> [5/6] Compilando instalador con Inno Setup 7..." -ForegroundColor Cyan
$ISCC = "C:\Program Files\Inno Setup 7\ISCC.exe"
if (-not (Test-Path $ISCC)) { throw "No se encontro ISCC.exe en $ISCC" }
$IssPath = Join-Path $RootDir "build\windows\installer\netpulse-inno.iss"
& $ISCC "/DMyAppVersion=$Version" $IssPath
if ($LASTEXITCODE -ne 0) { throw "ISCC.exe fallo con codigo $LASTEXITCODE" }

$SetupName = "NetPulse-v$Version-Setup.exe"
$SetupPath = Join-Path $ReleaseDir $SetupName
if (-not (Test-Path $SetupPath)) { throw "No se encontro $SetupPath" }

Write-Host "==> [6/6] Generando SHA256SUMS.txt..." -ForegroundColor Cyan
$Artifacts = @(
    @{ Path = $ExePath; Name = "NetPulse.exe" },
    @{ Path = $ZipPath; Name = $ZipName },
    @{ Path = $SetupPath; Name = $SetupName }
)

$SumLines = @()
$HashByName = @{}
foreach ($a in $Artifacts) {
    $hash = (Get-FileHash -Path $a.Path -Algorithm SHA256).Hash.ToLowerInvariant()
    $SumLines += "$hash  $($a.Name)"
    $HashByName[$a.Name] = $hash
}
$SumsPath = Join-Path $ReleaseDir "SHA256SUMS.txt"
Set-Content -Path $SumsPath -Value $SumLines -Encoding ASCII

# Sync real SHA-256 checksums into WinGet & Scoop manifests
$WingetInstaller = Join-Path $RootDir "packaging\winget\NetPulse.NetPulse.installer.yaml"
if (Test-Path $WingetInstaller) {
    $setupHashUpper = $HashByName[$SetupName].ToUpperInvariant()
    $wingetContent = (Get-Content -Path $WingetInstaller -Raw) -replace 'InstallerSha256:\s*[A-Za-z0-9_]+', "InstallerSha256: $setupHashUpper"
    Set-Content -Path $WingetInstaller -Value $wingetContent.TrimEnd() -Encoding UTF8
}
$ScoopManifest = Join-Path $RootDir "packaging\scoop\netpulse.json"
if (Test-Path $ScoopManifest) {
    $zipHashLower = $HashByName[$ZipName]
    $scoopContent = (Get-Content -Path $ScoopManifest -Raw) -replace '"hash":\s*"[^"]+"', "`"hash`": `"$zipHashLower`""
    Set-Content -Path $ScoopManifest -Value $scoopContent.TrimEnd() -Encoding UTF8
}

Write-Host "`n=== Release NetPulse v$Version lista ===" -ForegroundColor Green
foreach ($a in $Artifacts) {
    $sizeMB = [math]::Round((Get-Item $a.Path).Length / 1MB, 2)
    Write-Host ("  - {0,-35} {1,6} MB" -f $a.Name, $sizeMB)
}
Write-Host "`nSHA256SUMS.txt:" -ForegroundColor Yellow
Get-Content $SumsPath
