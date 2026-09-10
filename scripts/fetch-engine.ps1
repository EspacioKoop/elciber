# Descarga explícita del motor oficial. No instala servicios ni controladores.
# PowerShell 5.1+ / Windows x64. No necesita permisos de administrador.
param([string]$Destination = (Join-Path (Split-Path $PSScriptRoot -Parent) 'engines\easytier'))
$ErrorActionPreference = 'Stop'
$Version = '2.6.4'
$Expected = '27af91e270e554709b048bd32327fefd2dfce5062ae1e8701af7550c6f525f84'
$Uri = "https://github.com/EasyTier/EasyTier/releases/download/v$Version/easytier-windows-x86_64-v$Version.zip"
$Destination = [IO.Path]::GetFullPath($Destination)
if (Test-Path -LiteralPath $Destination) { throw 'El destino ya existe. Usa otro directorio; nunca se sobrescribe el motor instalado.' }
if (-not [Environment]::Is64BitOperatingSystem) { throw 'Este descargador requiere Windows x64.' }
$Parent = Split-Path $Destination -Parent
$null = New-Item -ItemType Directory -Force -Path $Parent
$Stage = Join-Path $Parent ('.elciber-engine-' + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $Stage
$ZipPath = Join-Path $Stage 'upstream.zip'
$Payload = Join-Path $Stage 'payload'
$null = New-Item -ItemType Directory -Path $Payload
try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $ZipPath -TimeoutSec 120
    if ((Get-Item -LiteralPath $ZipPath).Length -gt 167772160) { throw 'Paquete demasiado grande.' }
    if ((Get-FileHash -LiteralPath $ZipPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Expected) { throw 'SHA-256 incorrecto; no se instala nada.' }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $Archive = [IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        foreach ($Name in @('easytier-core.exe', 'easytier-cli.exe', 'wintun.dll')) {
            $Entries = @($Archive.Entries | Where-Object { $_.FullName -ceq "easytier-windows-x86_64/$Name" })
            if ($Entries.Count -ne 1 -or $Entries[0].Length -le 0 -or $Entries[0].Length -gt 167772160) { throw 'Estructura inesperada del paquete.' }
            # Lista explícita: no extraemos rutas arbitrarias del ZIP.
            [IO.Compression.ZipFileExtensions]::ExtractToFile($Entries[0], (Join-Path $Payload $Name), $false)
        }
    } finally { $Archive.Dispose() }
    if (Test-Path -LiteralPath $Destination) { throw 'El destino apareció durante la descarga; no se sobrescribe.' }
    [IO.Directory]::Move($Payload, $Destination)
    Write-Output "EasyTier $Version: SHA-256 verificado. No se ha ejecutado el motor ni instalado controladores."
} finally {
    # Solo el staging único creado por esta ejecución.
    if (Test-Path -LiteralPath $Stage) { Remove-Item -LiteralPath $Stage -Recurse -Force }
}
