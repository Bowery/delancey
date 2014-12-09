$packageName = "bowery-agent"
$url = "http://bowery.sh.s3.amazonaws.com/{{version}}_windows_386.zip"
$url64 = "http://bowery.sh.s3.amazonaws.com/{{version}}_windows_amd64.zip"

try {
  # Unzips and installs to pkg path, and adds links for binaries to a PATH directory.
  $root = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
  $chocTempDir = Join-Path $env:TEMP "chocolatey"
  $tempDir = Join-Path $chocTempDir "$packageName"
  New-Item "$tempDir" -Force -ItemType directory | Out-Null
  $file = Join-Path $tempDir "$($packageName)Install.zip"
  Get-ChocolateyWebFile "$packageName" "$file" "$url" "$url64"
  Get-ChocolateyUnzip "$file" "$root"

  # Install nssm if needed, the reason this isn't a hard dependency is that
  # nssm is wonky if two are installed and the dependency won't consider nssm
  # installed in places other than choco.
  if (!(Get-Command nssm -ErrorAction SilentlyContinue)) {
    cinst nssm
    if ($LastExitCode -gt 0) {
      throw "Command 'cinst nssm' returned $LastExitCode"
    }
  }

  # Reinstall the Windows Service.
  if (Get-Service "Bowery-Agent" -ErrorAction SilentlyContinue) {
    Start-ChocolateyProcessAsAdmin 'stop Bowery-Agent' net | Out-Null
    Start-ChocolateyProcessAsAdmin 'remove Bowery-Agent confirm' nssm | Out-Null
  }
  Start-ChocolateyProcessAsAdmin "install Bowery-Agent $root\bowery-agent.exe" nssm | Out-Null
  Start-ChocolateyProcessAsAdmin 'start Bowery-Agent' net | Out-Null

  Write-ChocolateySuccess "$packageName"
} catch {
  Write-ChocolateyFailure "$packageName" "$($_.Exception.Message)"
  throw
}
