If(-Not (Test-Path Env:PKG)){
  echo "cover only works with a single package, sorry"
  exit 255
}

$pkg = $Env:PKG

$coverout="coverage"

If (-Not (Test-Path $coverout)){
  mkdir $coverout
}ElseIf(-Not (Get-Item $coverout) -is [System.IO.DirectoryInfo]){
  mkdir $coverout
}

$coverpkg = $pkg.Trim("./")

$env:COVER="-coverprofile $coverout/$coverpkg.out"

& ./test.ps1

#if build did not succed we bail out
If (-Not($?)){
  echo "Could not run tests"
  exit 255
}

iex "go tool cover -html=`"$coverout/$coverpkg.out`""
