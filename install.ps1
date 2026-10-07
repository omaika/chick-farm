# Install piggery on Windows from a GitHub release: the binary for this CPU, checked against the
# release's checksums.txt, into $env:PIGGERY_INSTALL_DIR (default ~\.local\bin). No admin.
#
#   irm https://raw.githubusercontent.com/omaika/chicken-farm/main/install.ps1 | iex
#
# $env:PIGGERY_VERSION = "vX.Y.Z" installs that release instead of the latest.
& {
	$ErrorActionPreference = 'Stop'
	$ProgressPreference = 'SilentlyContinue' # Invoke-WebRequest is slow with its progress bar

	function Fail($msg) { throw "piggery install: $msg" }

	$repo = 'https://github.com/omaika/chicken-farm/releases'
	$dir = if ($env:PIGGERY_INSTALL_DIR) { $env:PIGGERY_INSTALL_DIR } else { Join-Path $HOME '.local\bin' }

	$cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
	switch ($cpu) {
		'AMD64' { $arch = 'amd64' }
		'ARM64' { $arch = 'arm64' }
		default { Fail "unsupported CPU ${cpu}: builds exist for amd64 and arm64" }
	}
	$name = "piggery-windows-$arch.exe"
	$url = if ($env:PIGGERY_VERSION) { "$repo/download/$($env:PIGGERY_VERSION)" } else { "$repo/latest/download" }

	$tmp = Join-Path ([IO.Path]::GetTempPath()) ("piggery-" + [guid]::NewGuid())
	New-Item -ItemType Directory -Path $tmp | Out-Null
	try {
		Write-Host "piggery install: downloading $name from $url"
		try { Invoke-WebRequest -UseBasicParsing -Uri "$url/$name" -OutFile (Join-Path $tmp $name) }
		catch { Fail "could not download $url/$name" }
		try { Invoke-WebRequest -UseBasicParsing -Uri "$url/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') }
		catch { Fail "could not download $url/checksums.txt" }

		$want = $null
		foreach ($line in Get-Content (Join-Path $tmp 'checksums.txt')) {
			$f = -split $line
			if ($f.Count -eq 2 -and ($f[1] -eq $name -or $f[1] -eq "*$name")) { $want = $f[0].ToLower() }
		}
		if (-not $want) { Fail "checksums.txt has no line for $name" }
		$got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $name)).Hash.ToLower()
		if ($got -ne $want) { Fail "checksum mismatch for $name (got $got, want $want); nothing installed" }

		New-Item -ItemType Directory -Force -Path $dir | Out-Null
		$exe = Join-Path $dir 'piggery.exe'
		# A running piggery.exe cannot be overwritten, but it can be renamed: it keeps running from
		# piggery.exe.old (a unique name while an older one still runs from there), which a later
		# install or `piggery update` removes.
		Get-ChildItem -Path $dir -Filter 'piggery.exe.old*' -ErrorAction SilentlyContinue |
			Remove-Item -Force -ErrorAction SilentlyContinue
		$old = "$exe.old"
		if (Test-Path $old) { $old = "$exe.old-" + [DateTime]::UtcNow.Ticks }
		if (Test-Path $exe) { Move-Item $exe $old }
		Copy-Item (Join-Path $tmp $name) $exe
		Remove-Item -Force -ErrorAction SilentlyContinue $old
	}
	finally {
		Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $tmp
	}

	$version = try { & $exe --version 2>$null } catch { $name }
	Write-Host "piggery install: installed $version in $dir"
	$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
	if (-not (($env:Path + ';' + $userPath) -split ';' | Where-Object { $_ -and ($_.TrimEnd('\') -ieq $dir.TrimEnd('\')) })) {
		Write-Host "piggery install: $dir is not on your PATH; add it for your user with:"
		Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"$dir;`" + [Environment]::GetEnvironmentVariable('Path', 'User'), 'User')"
	}
	Write-Host 'Next: on a new machine, piggery setup <harness> (pi, claude, codex, omp, dsh or paseo); upgrading, piggery setup --outdated'
}
