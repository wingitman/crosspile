#Requires -Version 5.1
param(
    [switch]$Default,
    [switch]$BuildAll
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$BinaryName = 'crosspile.exe'
$Remote     = 'https://github.com/wingitman/crosspile.git'
$SourceRoot = $PSScriptRoot
$BuildDir   = Join-Path $SourceRoot 'bin'
$BuildBin   = Join-Path ([System.IO.Path]::GetTempPath()) ("crosspile-{0}.exe" -f [guid]::NewGuid())
$ReleaseBin = Join-Path $SourceRoot 'releases\windows\crosspile.exe'
$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\crosspile'
$DestBin    = Join-Path $InstallDir $BinaryName

function Step([string]$msg) { Write-Host "==> $msg" -ForegroundColor Cyan }
function Note([string]$msg) { Write-Host "    $msg" -ForegroundColor Yellow }
function Ok([string]$msg)   { Write-Host "    $msg" -ForegroundColor Green }
function GitQuiet([string[]]$gitArgs) {
    $old = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $out = & git @gitArgs 2>$null
        $script:LastGitQuietExitCode = $LASTEXITCODE
        if ($LASTEXITCODE -ne 0) { return '' }
        return (($out -join "`n") -replace "`r", '').Trim()
    } finally { $ErrorActionPreference = $old }
}

if (-not $BuildAll -and (Test-Path (Join-Path $SourceRoot '.git'))) {
    Step 'Checking git remote for updates...'
    $RemoteUrl = GitQuiet @('-C', $SourceRoot, 'remote', 'get-url', 'origin')
    if (-not $RemoteUrl) { $RemoteUrl = $Remote }
    $env:GIT_TERMINAL_PROMPT = '0'
    $env:GCM_INTERACTIVE = 'never'
    GitQuiet @('-C', $SourceRoot, 'fetch', 'origin', '--quiet') | Out-Null
    $RemoteCommit = ''
    foreach ($ref in @('@{u}', 'origin/HEAD', 'origin/main', 'origin/master')) {
        $RemoteCommit = GitQuiet @('-C', $SourceRoot, 'rev-parse', $ref)
        if ($RemoteCommit) { break }
    }
    if (-not $RemoteCommit) {
        $line = GitQuiet @('ls-remote', $RemoteUrl, 'HEAD')
        if ($line) { $RemoteCommit = ($line -split '\s+')[0] }
    }
    $env:GIT_TERMINAL_PROMPT = $null
    $env:GCM_INTERACTIVE = $null
    $LocalCommit = GitQuiet @('-C', $SourceRoot, 'rev-parse', 'HEAD')
    if ($RemoteCommit -and $LocalCommit -and -not $RemoteCommit.StartsWith($LocalCommit)) {
        Note "Local : $($LocalCommit.Substring(0, [Math]::Min(7, $LocalCommit.Length)))"
        Note "Remote: $($RemoteCommit.Substring(0, [Math]::Min(7, $RemoteCommit.Length)))"
        $pull = Read-Host '    Pull latest changes before installing? [Y/n]'
        if ($pull -eq '' -or $pull -match '^[Yy]') {
            $env:GCM_INTERACTIVE = 'never'
            & git -C $SourceRoot pull
            $env:GCM_INTERACTIVE = $null
            if ($LASTEXITCODE -ne 0) { throw 'git pull failed' }
        }
    } else { Ok 'Already up to date or remote unavailable.' }
}

if ($BuildAll) {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go is required for -BuildAll' }
    $Commit = GitQuiet @('-C', $SourceRoot, 'rev-parse', '--short', 'HEAD')
    $Origin = GitQuiet @('-C', $SourceRoot, 'remote', 'get-url', 'origin')
    if (-not $Origin) { $Origin = $Remote }
    $LdFlags = "-s -w -X 'main.version=$Commit' -X 'main.origin=$Origin' -X 'main.repoDir=$SourceRoot'"
    $targets = @(
        @{ GOOS='linux'; GOARCH='amd64'; Out='releases\linux\amd64\crosspile' },
        @{ GOOS='linux'; GOARCH='arm64'; Out='releases\linux\arm64\crosspile' },
        @{ GOOS='darwin'; GOARCH='amd64'; Out='releases\darwin\amd64\crosspile' },
        @{ GOOS='darwin'; GOARCH='arm64'; Out='releases\darwin\arm64\crosspile' },
        @{ GOOS='windows'; GOARCH='amd64'; Out='releases\windows\crosspile.exe' }
    )
    foreach ($t in $targets) {
        Step "Building $($t.GOOS)/$($t.GOARCH)"
        $out = Join-Path $SourceRoot $t.Out
        New-Item -ItemType Directory -Path (Split-Path $out) -Force | Out-Null
        $env:GOOS = $t.GOOS; $env:GOARCH = $t.GOARCH
        & go -C $SourceRoot build -ldflags $LdFlags -o $out .
        if ($LASTEXITCODE -ne 0) { throw "build failed for $($t.GOOS)/$($t.GOARCH)" }
    }
    $env:GOOS = $null; $env:GOARCH = $null
    exit 0
}

if (Get-Command go -ErrorAction SilentlyContinue) {
    Step 'Building from source...'
    $Commit = GitQuiet @('-C', $SourceRoot, 'rev-parse', '--short', 'HEAD')
    if (-not $Commit) { $Commit = 'unknown' }
    $Origin = GitQuiet @('-C', $SourceRoot, 'remote', 'get-url', 'origin')
    if (-not $Origin) { $Origin = $Remote }
    $LdFlags = "-s -w -X 'main.version=$Commit' -X 'main.origin=$Origin' -X 'main.repoDir=$SourceRoot'"
    & go -C $SourceRoot build -ldflags $LdFlags -o $BuildBin .
    if ($LASTEXITCODE -ne 0) { throw 'go build failed' }
    $SourceBin = $BuildBin
} else {
    Step 'Go not found - using pre-built binary.'
    if (-not (Test-Path $ReleaseBin)) { throw "missing pre-built binary: $ReleaseBin" }
    $SourceBin = $ReleaseBin
    $Commit = GitQuiet @('-C', $SourceRoot, 'rev-parse', '--short', 'HEAD')
    $Origin = GitQuiet @('-C', $SourceRoot, 'remote', 'get-url', 'origin')
    if (-not $Origin) { $Origin = $Remote }
}

Step 'Installing binary...'
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item -Path $SourceBin -Destination $DestBin -Force
if ((Test-Path $BuildBin) -and $SourceBin -eq $BuildBin) { Remove-Item -Path $BuildBin -Force }
Ok "Installed: $DestBin"

Step 'Recording repository location for auto-updater...'
if (-not $Commit) { $Commit = 'unknown' }
& $DestBin --set-repo-dir $SourceRoot $Origin $Commit 'git' 'windows/amd64'
if ($LASTEXITCODE -ne 0) { Note 'Warning: could not write install metadata. Auto-updates may not work.' }

Step 'Updating user PATH...'
$currentPath = (Get-ItemProperty -Path 'HKCU:\Environment' -Name Path -ErrorAction SilentlyContinue).Path
if (-not $currentPath -or ($currentPath -split ';') -notcontains $InstallDir) {
    $newPath = if ($currentPath) { "$currentPath;$InstallDir" } else { $InstallDir }
    Set-ItemProperty -Path 'HKCU:\Environment' -Name Path -Value $newPath
    Ok "Added $InstallDir to user PATH."
} else { Note "$InstallDir is already in your PATH." }

if ($Default) { & $DestBin --default }

Write-Host ''
Write-Host '  crosspile installed successfully!' -ForegroundColor Green
Write-Host '  Open a new terminal and run: crosspile' -ForegroundColor Cyan
