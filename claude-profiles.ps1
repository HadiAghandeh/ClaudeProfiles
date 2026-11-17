<#
Claude Desktop and Claude Code profile launcher.

Shows every Claude profile (user) you have, lets you add a new one by email,
and starts the one you pick with its own login, settings and data:
  - Claude Desktop in its own window (--user-data-dir)
  - Claude Code in its own terminal (CLAUDE_CONFIG_DIR)

Double-click start.bat to open the menu.

Command line (for shortcuts):
  start.bat                                      open the menu
  start.bat -Profile a@second.com                start that user's Claude Desktop, no menu
  start.bat -Profile a@second.com -Code          start that user's Claude Code in the current folder
  start.bat -Profile a@second.com -Code -Dir C:\src\app   ...in that folder
  start.bat -List                                list the profiles and exit
#>
param(
    [string]$Profile,
    [switch]$Code,
    [string]$Dir,
    [switch]$List
)

$ErrorActionPreference = 'Stop'

# ---------------------------------------------------------------- settings
$DataRoot     = Join-Path $env:LOCALAPPDATA 'Claude-Data'
$CodeDataRoot = Join-Path $env:LOCALAPPDATA 'Claude-Code-Data'
$RecordsFile  = Join-Path $PSScriptRoot 'profiles.json'
$ClaudeCodeCandidates = @(
    (Join-Path $env:USERPROFILE '.local\bin\claude.exe'),
    (Join-Path $env:APPDATA 'npm\claude.cmd')
)
$ClaudeCandidates = @(
    (Join-Path $env:LOCALAPPDATA 'Microsoft\WindowsApps\claude-desktop.exe'),
    (Join-Path $env:LOCALAPPDATA 'Programs\Claude\Claude.exe'),
    (Join-Path $env:LOCALAPPDATA 'AnthropicClaude\Claude.exe')
)

# ---------------------------------------------------------------- helpers
function Find-Claude {
    foreach ($c in $ClaudeCandidates) { if (Test-Path $c) { return $c } }
    throw "Claude Desktop was not found. Install it from the Microsoft Store, or add its Claude.exe path to `$ClaudeCandidates in $PSCommandPath."
}

function Find-ClaudeCode {
    $cmd = Get-Command claude -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($cmd) { return $cmd.Source }
    foreach ($c in $ClaudeCodeCandidates) { if (Test-Path $c) { return $c } }
    throw "Claude Code was not found. Install it (see https://docs.claude.com/en/docs/claude-code/setup), or add its claude.exe path to `$ClaudeCodeCandidates in $PSCommandPath."
}

function Get-CodeFolder($entry) {
    # Claude Code config lives next to the Desktop data, in a parallel root with the same folder name
    return (Join-Path $CodeDataRoot (Split-Path -Leaf $entry.folder))
}

function Quote-PS([string]$s) { return "'" + ($s -replace "'", "''") + "'" }

function Get-SafeFolderName([string]$label) {
    $safe = ($label.Trim() -replace '[^A-Za-z0-9._-]', '_').Trim('._-')
    if (-not $safe) { $safe = 'profile' }
    return $safe
}

function Load-Records {
    if (Test-Path $RecordsFile) {
        $raw = Get-Content -Raw -Path $RecordsFile
        if ($raw.Trim()) {
            # ForEach-Object unrolls the array ConvertFrom-Json hands back as one object,
            # and keeps only real entries
            return @($raw | ConvertFrom-Json | ForEach-Object { $_ } | Where-Object { $_.PSObject.Properties['label'] })
        }
    }
    return @()
}

function Save-Records($records) {
    $json = ConvertTo-Json -InputObject @($records) -Depth 4
    Set-Content -Path $RecordsFile -Value $json -Encoding UTF8
}

function Is-MainStoreFolder([string]$name) {
    # The Microsoft Store build keeps the main profile in a folder like "claude_pzs8sxrjxfjjc"
    return ($name -match '^claude_[a-z0-9]+$')
}

function Discover-Profiles($records) {
    # Any folder in Claude-Data that is not the main Store profile and is not yet
    # recorded becomes a profile named after its folder, so nothing already on
    # disk is hidden from the menu.
    $records = @($records | Where-Object { $_ })   # an empty list arrives as $null in PowerShell 5.1
    if (-not (Test-Path $DataRoot)) { return @($records) }
    $known = @($records | ForEach-Object { "$($_.folder)".ToLower() })
    $list  = [System.Collections.ArrayList]@($records)
    Get-ChildItem -Path $DataRoot -Directory | ForEach-Object {
        if (Is-MainStoreFolder $_.Name) { return }
        if ($known -contains $_.FullName.ToLower()) { return }
        [void]$list.Add([pscustomobject]@{
            label    = $_.Name
            folder   = $_.FullName
            created  = $_.CreationTime.ToString('yyyy-MM-dd')
            lastUsed = ''
        })
    }
    return @($list)
}

function Launch-Profile($records, $entry) {
    $claude = Find-Claude
    if (-not (Test-Path $entry.folder)) { New-Item -ItemType Directory -Path $entry.folder | Out-Null }
    Start-Process -FilePath $claude -ArgumentList ('--user-data-dir="{0}"' -f $entry.folder)
    $entry.lastUsed = (Get-Date).ToString('yyyy-MM-dd HH:mm')
    Save-Records $records
    Write-Host ("Started Claude Desktop as {0}" -f $entry.label) -ForegroundColor Green
}

function Launch-Code($records, $entry, [string]$workDir) {
    $claude = Find-ClaudeCode
    $configDir = Get-CodeFolder $entry
    if (-not (Test-Path $configDir)) { New-Item -ItemType Directory -Path $configDir -Force | Out-Null }
    if (-not $workDir) { $workDir = $env:USERPROFILE }
    if (-not (Test-Path $workDir -PathType Container)) { throw "Folder not found: $workDir" }
    $workDir = (Resolve-Path $workDir).Path

    $title = "Claude Code - $($entry.label)"
    $inner = "`$Host.UI.RawUI.WindowTitle = $(Quote-PS $title); " +
             "`$env:CLAUDE_CONFIG_DIR = $(Quote-PS $configDir); " +
             "Set-Location -LiteralPath $(Quote-PS $workDir); " +
             "& $(Quote-PS $claude)"
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($inner))
    $psArgs  = @('-NoExit', '-NoProfile', '-EncodedCommand', $encoded)

    $wt = Get-Command wt.exe -ErrorAction SilentlyContinue
    if ($wt) {
        Start-Process -FilePath $wt.Source -ArgumentList (@('new-tab', '--title', "`"$title`"", 'powershell.exe') + $psArgs)
    } else {
        Start-Process -FilePath 'powershell.exe' -ArgumentList $psArgs
    }
    $entry.lastUsed = (Get-Date).ToString('yyyy-MM-dd HH:mm')
    Save-Records $records
    Write-Host ("Started Claude Code as {0} in {1}" -f $entry.label, $workDir) -ForegroundColor Green
}

function Pick-Code($records, [int]$i) {
    if ($i -lt 0 -or $i -ge $records.Count) { Write-Host 'No such number.' -ForegroundColor Yellow; Start-Sleep 1; return $false }
    $dirIn = (Read-Host 'Project folder for Claude Code (blank = your home folder)').Trim().Trim('"')
    Launch-Code $records $records[$i] $dirIn
    return $true
}

function Launch-Main {
    $claude = Find-Claude
    Start-Process -FilePath $claude
    Write-Host 'Started your main Claude Desktop' -ForegroundColor Green
}

# Returns the updated records. Sets $script:Launched when it started a window.
function Add-Profile($records) {
    $label = (Read-Host 'Email or name for the new user (blank to cancel)').Trim()
    if (-not $label) { return @($records) }
    if ($records | Where-Object { $_.label -ieq $label }) {
        Write-Host "A user named $label already exists." -ForegroundColor Yellow
        Start-Sleep 2
        return @($records)
    }
    $safe   = Get-SafeFolderName $label
    $folder = Join-Path $DataRoot $safe
    $n = 2
    while (Test-Path $folder) { $folder = Join-Path $DataRoot ("{0}-{1}" -f $safe, $n); $n++ }
    New-Item -ItemType Directory -Path $folder | Out-Null
    $entry = [pscustomobject]@{
        label    = $label
        folder   = $folder
        created  = (Get-Date).ToString('yyyy-MM-dd')
        lastUsed = ''
    }
    $records = @($records) + $entry
    Save-Records $records
    Write-Host ("Added {0}  ->  {1}" -f $label, $folder) -ForegroundColor Green
    $go = Read-Host 'Start it now and sign in? [D]esktop / [C]ode / [N]o (default D)'
    if ($go -eq '' -or $go -match '^[DdYy]') {
        Launch-Profile $records $entry
        $script:Launched = $true
    } elseif ($go -match '^[Cc]') {
        $script:Launched = Pick-Code $records ($records.Count - 1)
    }
    return @($records)
}

function Remove-Profile($records) {
    $pick = Read-Host 'Number of the user to remove (blank to cancel)'
    if (-not ($pick -match '^\d+$')) { return @($records) }
    $i = [int]$pick - 1
    if ($i -lt 0 -or $i -ge $records.Count) { Write-Host 'No such number.' -ForegroundColor Yellow; return @($records) }
    $entry = $records[$i]
    $codeFolder = Get-CodeFolder $entry
    Write-Host ("This removes {0} and deletes its data folders:" -f $entry.label)
    Write-Host ("  {0}   (Claude Desktop)" -f $entry.folder)
    Write-Host ("  {0}   (Claude Code)" -f $codeFolder)
    Write-Host 'That signs the user out on this PC and cannot be undone. Conversations stay on the Claude account.'
    $wipe = Read-Host 'Type DELETE to confirm, anything else to cancel'
    if ($wipe -ne 'DELETE') { Write-Host 'Cancelled.'; return @($records) }
    if (Test-Path $entry.folder) { Remove-Item -Recurse -Force -Path $entry.folder }
    if (Test-Path $codeFolder)   { Remove-Item -Recurse -Force -Path $codeFolder }
    $records = @($records | Where-Object { $_ -ne $entry })
    Save-Records $records
    Write-Host ("Removed {0}." -f $entry.label) -ForegroundColor Green
    return @($records)
}

function Show-Menu($records) {
    Clear-Host
    Write-Host ''
    Write-Host '  Claude - choose a user' -ForegroundColor Cyan
    Write-Host '  ----------------------'
    if ($records.Count -eq 0) {
        Write-Host '  (no extra users yet - press A to add one)'
    }
    $i = 1
    foreach ($r in $records) {
        $when = if ($r.lastUsed) { "last used $($r.lastUsed)" } else { 'never used' }
        Write-Host ("  {0,2})  {1,-32} {2}" -f $i, $r.label, $when)
        $i++
    }
    Write-Host ''
    Write-Host '   1, 2, ...   Claude Desktop as that user'
    Write-Host '  C1, C2, ...  Claude Code as that user (opens a terminal)'
    Write-Host ''
    Write-Host '   M)  Main Claude Desktop (your normal profile)'
    Write-Host '   A)  Add a new user'
    Write-Host '   R)  Remove a user'
    Write-Host '   Q)  Quit'
    Write-Host ''
}

# ---------------------------------------------------------------- main
$script:Launched = $false
$records = @(Discover-Profiles @(Load-Records))
Save-Records $records

if ($List) {
    $records | Select-Object label, folder, @{ n = 'codeFolder'; e = { Get-CodeFolder $_ } }, created, lastUsed | Format-Table -AutoSize
    exit 0
}

if ($Profile) {
    $entry = $records | Where-Object { $_.label -ieq $Profile } | Select-Object -First 1
    if (-not $entry) { throw "No user named '$Profile'. Run start.bat without arguments to see the list or add it." }
    if ($Code) {
        $workDir = if ($Dir) { $Dir } else { (Get-Location).Path }
        Launch-Code $records $entry $workDir
    } else {
        Launch-Profile $records $entry
    }
    exit 0
}

while ($true) {
    Show-Menu $records
    $choice = (Read-Host 'Type a number or letter, then Enter').Trim()
    switch -Regex ($choice) {
        '^[Qq]$' { exit 0 }
        '^[Mm]$' { Launch-Main; exit 0 }
        '^[Aa]$' { $records = Add-Profile $records; if ($script:Launched) { exit 0 } }
        '^[Rr]$' { $records = Remove-Profile $records; Read-Host 'Press Enter to continue' | Out-Null }
        '^[Cc]\s*\d+$' {
            $n = [int]($choice -replace '[^\d]', '')
            if (Pick-Code $records ($n - 1)) { exit 0 }
        }
        '^\d+$'  {
            $i = [int]$choice - 1
            if ($i -ge 0 -and $i -lt $records.Count) { Launch-Profile $records $records[$i]; exit 0 }
            Write-Host 'No such number.' -ForegroundColor Yellow
            Start-Sleep 1
        }
        default  { }
    }
}
