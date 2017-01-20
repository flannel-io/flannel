#This integration test will:
#1. Download etcd binaries and extract them to the current folder
#2. Start an etcd server
#3. Put the initial configuration for flannel in the etcd db
#4. Start a flannel server for faking external subnet requests
#5. Start flannel
#6. Request for 2 additional subnets
#7. Delete the two additional subnets
#8. Clean-up
$fakeIP1 = "10.12.10.10"
$fakeIP2 = "10.13.10.10"
$etcdctl = "./etcdctl.exe"
$etcd = "./etcd.exe"
$etcdVersion = "etcd-v2.3.7-windows-amd64"
$flanneld = "./bin/flanneld"

function Add-Lease($leaseIp){
  #Request a fake lease so that we can check that a route with this IP will be found in the routing table
  $curlAdd = -join("curl -L -H `"ContentType: application/json`" -X POST http://",$ip,":8888/v1/_/leases -d `"{\`"PublicIP\`":\`"$leaseIP\`",\`"BackendType\`":\`"host-gw\`"}`"")
  Write-Host "Running $curlAdd"
  $addedSubnet = cmd /C $curlAdd

  If (-Not($?)){
    Write-Host "Could not request subnet from flanneld server"
    exit 255
  }

  $val = ConvertFrom-json $addedSubnet
  $searchTerm = $val.Subnet

  #see if the lease is found in the routing table
  Write-Host "Looking for $searchTerm"
  sleep -s 2
  $subnet=Get-NetRoute -DestinationPrefix $val.Subnet

  if(!$subnet){
    Write-Host "The subnet $subnet was not found in the routing table"
    Get-NetRoute
    exit 255
  }

  return $val.Subnet
}

function Delete-Lease($subnet){
  #Delete the lease so that we can check that we no longer have the lease in the roouting table
  $subnet_ = $subnet -replace "/", "-"
  $curlDelete = -join("curl -L -X DELETE http://",$ip,":8888/v1/_/leases/$subnet_")
  Write-Host "Running $curlDelete"
  $deletedSubnet = cmd /C $curlDelete

  If (-Not($?)){
    Write-Host "Could not delete subnte from to flanneld server"
    exit 255
  }

  sleep -s 2
  Write-Host "The $subnet subnet should not be found in the routing table."
  $rez = Get-NetRoute | where {$_.DestinationPrefix -eq "$subnet"}

  #if we still have the route in the routing table than we have a problem
  if ($rez){
    Write-Host "The subnet $subnet was still found in the routing table"
    Get-NetRoute
    exit 255
  }
}

function Get-MyExternalIP{
  $rez = Get-NetIPAddress -AddressFamily IPv4 -AddressState Preferred -InterfaceAlias Ethernet* | Select-Object -First 1 | Format-Wide -Property IPAddress

  if (!$rez){
    Write-Host "Could not get IP Address"
    exit 255
  }

  if($rez.Length -ne 5){
    Write-Host "Could not get the correct response while trying to get the IP Address"
    exit 255
  }

  $rez = $rez[2].formatEntryInfo.formatPropertyField.propertyValue.ToString()

  return $rez
}

Write-Host "Getting IP address"

$ip = Get-MyExternalIP

Write-Host "Using $ip"

& ./build.ps1

#if build did not succed we bail out
If (-Not($?)){
  exit 255
}

#1. Download etcd binaries and extract them to the current folder
$downloadEtcd = "curl -k -L https://github.com/coreos/etcd/releases/download/v2.3.7/$etcdVersion.zip -o $etcdVersion.zip"
cmd /C $downloadEtcd
if (-not (Test-Path "./$etcdVersion.zip")){
  Write-Host "Could not download etcd"
  exit 255
}

cmd /C "7z x -y $etcdVersion.zip"
cmd /C "copy $etcdVersion\etcd.exe etcd.exe"
cmd /C "copy $etcdVersion\etcdctl.exe etcdctl.exe"

#2. Start an etcd server
$etcdCmd = -join("--initial-cluster-token etcd-cluster --advertise-client-urls http://",$ip,":2379 --listen-peer-urls http://",$ip,":2380 --listen-client-urls http://",$ip,":2379")
echo "Running $etcd $etcdCmd"
$etcdProcess = Start-Process "$etcd" -Args "$etcdCmd" -PassThru
If (-Not($?)){
  Write-Host "Could not start etcd server"
  exit 255
}

#We wait for etcd to come online
sleep -s 10


#3. Put the initial configuration for flannel in the etcd db
$etcdSetCmd = -join ("$etcdctl --endpoints=http://",$ip,":2379 set /coreos.com/network/config '{\`"Network\`":\`"10.0.0.0/8\`", \`"SubnetLen\`":20, \`"SubnetMin\`":\`"10.10.0.0\`", \`"SubnetMax\`":\`"10.99.0.0\`", \`"Backend\`": { \`"Type\`":\`"host-gw\`"}}'")
echo "Running $etcdSetCmd :)"
iex $etcdSetCmd

If (-Not($?)){
  Write-Host "Could not set subnet values in etcd"
  exit 255
}

#4. Start a flannel server for faking external subnet requests
#start flanneld as server so that we can add new routes through curl
$fServerCmd = -join ("--etcd-endpoints=http://",$ip,":2379 --listen :8888")
echo "Running $flanneld $fServerCmd"
$flannelServerProcess = Start-Process "$flanneld" -Args "$fServerCmd" -PassThru

If (-Not($?)){
  Write-Host "Could not start flanneld server"
  exit 255
}

#5. Start flannel
#start flanneld - this one will add routes to the routing table
$fCmd = -join ("--etcd-endpoints=http://"+$ip+":2379")
Write-Host "Running $flanneld $fCmd"
$flannelProcess = Start-Process "$flanneld" -Args "$fCmd" -PassThru

If (-Not($?)){
  Write-Host "Could not start flanneld"
  exit 255
}

#6. Request for 2 additional subnets
#Request a fake lease so that we can check that a route with this IP will be found in the routing table
$subnet1 = Add-Lease $fakeIP1
$subnet2 = Add-Lease $fakeIP2

#7. Delete the two additional subnets
Delete-Lease $subnet1
Delete-Lease $subnet2

Write-Host "Success"

#8. Clean-up
#Stop the started processes and delete the used files.
Stop-Process $flannelProcess
Stop-Process $flannelServerProcess
Stop-Process $etcdProcess
cmd /C "del etcd.exe"
cmd /C "del etcdctl.exe"
cmd /C "del $etcdVersion.zip"
cmd /C "rmdir /S /Q $etcdVersion"
cmd /C "rmdir /S /Q default.etcd"
