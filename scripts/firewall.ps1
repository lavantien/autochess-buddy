#Requires -Version 7
#Requires -RunAsAdministrator
# inbound allow rule for the built server exe (make build). loopback traffic
# never hits windows firewall, so this only matters when serving on a
# non-loopback addr like -addr 0.0.0.0:8080.
[CmdletBinding()]
param(
    [int[]]$Port = 8080,
    [string]$ExePath = (Join-Path $PSScriptRoot '..\autochess.exe'),
    [switch]$Remove
)

$name = 'autochess-buddy'
if ($Remove) {
    Get-NetFirewallRule -DisplayName $name -ErrorAction Ignore | Remove-NetFirewallRule
    Write-Output "removed firewall rule '$name'"
    exit 0
}

$exe = (Resolve-Path $ExePath).ProviderPath
New-NetFirewallRule -DisplayName $name -Direction Inbound -Program $exe -Protocol TCP -LocalPort $Port -Action Allow | Out-Null
Write-Output "allowed inbound tcp port(s) $($Port -join ', ') for $exe"
