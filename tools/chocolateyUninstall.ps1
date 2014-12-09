Write-Host "Stopping and removing Windows Service 'Bowery Agent'."
if (Get-Service "Bowery-Agent" -ErrorAction SilentlyContinue) {
  Start-ChocolateyProcessAsAdmin 'stop Bowery-Agent' net | Out-Null
  Start-ChocolateyProcessAsAdmin 'remove Bowery-Agent confirm' nssm | Out-Null
}
