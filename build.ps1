function LinkerArgs([string]$key, [string]$value) {
    $goversion = go version
    $regex = "go([0-9]+).([0-9]+)."

    if($goversion -match $regex) {
        $gomajor = $matches[1]
        $gominor = $matches[2]
        if($gomajor -eq "1" -and $gominor -eq "4") {             
            "{0} {1}" -f $key, $value
        } else {
            "{0}={1}" -f $key, $value 
        }        
    } else {
        Write-Host "Unable to determine go version, cannot continue!"
        exit 1
    }
}

$version = git describe --dirty
$versionarg = LinkerArgs "github.com/coreos/flannel/version.Version" $version
$orgpath = "github.com/coreos"
$repopath = "{0}/flannel" -f $orgpath
$gldflags = "-X {0}" -f $versionarg

$goenvoutput = go env
foreach($x in $goenvoutput) {
    if($x -match "set GOOS=(.*)") {
        if($matches[1] -ne "windows") {
            Write-Host "Not on Windows, cannot continue"
            exit 1
        }
    }
}

$gobin = "{0}\\bin" -f $pwd
$gopath = "{0}\\gopath" -f $pwd
$binary = "{0}\\flanneld.exe" -f $gobin

Write-Host "Building flanneld..."
go build -o $binary -ldflags "$gldflags" $repopath    
