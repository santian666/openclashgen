$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path

function Join-RootPath([string]$Path) {
  if ([System.IO.Path]::IsPathRooted($Path)) {
    return $Path
  }
  return Join-Path $Root $Path
}

function New-ConfigFromExample([string]$Target) {
  $TargetPath = Join-RootPath $Target
  $PrefDir = Split-Path -Parent $TargetPath
  if ($PrefDir) {
    New-Item -ItemType Directory -Path $PrefDir -Force | Out-Null
  }

  $Extension = [System.IO.Path]::GetExtension($TargetPath).ToLowerInvariant()
  $ExampleName = switch ($Extension) {
    ".yml" { "pref.example.yml"; break }
    ".yaml" { "pref.example.yml"; break }
    ".ini" { "pref.example.ini"; break }
    default { "pref.example.toml"; break }
  }

  $Example = Join-Path $Root "base\$ExampleName"
  if (Test-Path $Example) {
    Copy-Item $Example $TargetPath
    return $TargetPath
  }

  throw "Cannot create configuration file '$TargetPath'. Missing '$Example'."
}

function Resolve-PrefPath {
  if ($env:PREF_PATH) {
    $Target = Join-RootPath $env:PREF_PATH
    if (-not (Test-Path $Target)) {
      return New-ConfigFromExample $Target
    }
    return $Target
  }

  foreach ($Name in @("pref.toml", "pref.yml", "pref.ini")) {
    $Candidate = Join-Path $Root "base\$Name"
    if (Test-Path $Candidate) {
      return $Candidate
    }
  }

  foreach ($Pair in @(
    @{ Example = "pref.example.toml"; Target = "pref.toml" },
    @{ Example = "pref.example.yml"; Target = "pref.yml" },
    @{ Example = "pref.example.ini"; Target = "pref.ini" }
  )) {
    $Example = Join-Path $Root ("base\" + $Pair.Example)
    if (Test-Path $Example) {
      $Target = Join-Path $Root ("base\" + $Pair.Target)
      Copy-Item $Example $Target
      return $Target
    }
  }

  throw "No configuration file found. Expected base\pref.toml, base\pref.yml, or base\pref.ini."
}

$PrefPath = Resolve-PrefPath
& (Join-Path $Root "subconverter.exe") -f $PrefPath
exit $LASTEXITCODE
