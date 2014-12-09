# Usage: powershell -ExecutionPolicy unrestricted -File .\release_agent.ps1
#        .\release_agent.ps1(while in powershell cli)
$ErrorActionPreference = "Stop"
$origpwd = $pwd

# Get and cd into the directory containing this script.
$cmd = $MyInvocation.MyCommand
$root = $cmd.Definition.Replace($cmd.Name, "..")

# Build utility for zip aws upload.
$scripts = "$root\..\desktop\scripts"
cd $scripts
go build util.go
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}
cd $root

# Retrieve the version.
$version = Get-Content VERSION
echo "Version: $version"

# Build the agent.
go get -u github.com/laher/goxc
if ($LastExitCode -gt 0) {
    cd $origpwd
    exit 1
}
goxc -tasks-=validate "-d=$root\pkg" "-pv=$version" "-arch=386 amd64" -os=windows xc
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}

# Zip up the binaries.
New-Item "$root\pkg\$version\dist" -Force -ItemType directory
ForEach ($platform in Get-ChildItem "$root\pkg\$version") {
  $name = $platform.name
  if ($name -eq "dist") {
    continue
  }

  Remove-Item "$root\pkg\$version\$name\bowery-agent.exe" -Force -ErrorAction SilentlyContinue
  Rename-Item "$root\pkg\$version\$name\agent.exe" "bowery-agent.exe" -Force
  "$scripts\util.exe" zip "$root\pkg\$version\$name" "$root\pkg\$version\dist\$($version)_$name.zip"
  if ($LastExitCode -gt 0) {
    cd $origpwd
    exit 1
  }
}

# Pack the choco pkg.
Copy-Item "$root\icon.png" "$root\pkg\$version\dist" -Force
New-Item "$root\choco\tools" -Force -ItemType directory
New-Item "$root\choco\bowery-agent.nuspec" -Force -ItemType file -Value ""
$nuspec = (Get-Content "$root\bowery-agent.nuspec") -replace "{{version}}", "$version"
[System.IO.File]::WriteAllLines("$root\choco\bowery-agent.nuspec", $nuspec)
ForEach ($file in Get-ChildItem "$root\tools") {
  $content = (Get-Content "$root\tools\$($file.name)") -replace "{{version}}", "$version"
  [System.IO.File]::WriteAllLines("$root\choco\tools\$($file.name)", $content)
}
cd "$root\choco"
cpack
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}

# Push the choco pkg.
cinst nuget.commandline
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}
nuget setApiKey "2e664545-c457-4d43-afc2-6faa65203bf4" -Source "http://chocolatey.org/"
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}
cpush "bowery-agent.$($version).nupkg"
if ($LastExitCode -gt 0) {
  cd $origpwd
  exit 1
}

cd $root
"$scripts\util.exe" aws "bowery.sh" "$root\pkg\$version\dist"
cd $origpwd
