& ./build.ps1

#if build did not succed we bail out
If (-Not($?)){
  exit 255
}

#we double check to see if the executable was generated
If(-Not (Test-Path "./bin/flanneld.exe")){
  echo "Could not find executable"
  exit 255
}

$COVER="-cover"
If(Test-Path Env:COVER){
  $COVER = $ENV:COVER
}

$testable = $formatable = @("pkg/ip","subnet","network","remote")

$orgpath = "github.com/coreos"
$repopath = "{0}/flannel" -f $orgpath

#user has provided PKG override
If(Test-Path Env:PKG){
  $pkg = $Env:PKG
  $testable = $pkg.Trim("./")
  $formatable = $testable
}
$test = ""
$testable | foreach{ $test+="`"$repopath/{0}`" " -f $_ }
echo "Running tests for $test"

iex "go test -i $test"
iex "go test $COVER $test"
