$fakeIP = "10.12.10.10"
$etcdctl = "./etcdctl.exe"
$etcd = "./etcd.exe"
$etcdVersion = "etcd-v2.3.7-windows-amd64"
$flanneld = "./bin/flanneld"

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

#download etcd
$downloadEtcd = "curl -k -L https://github.com/coreos/etcd/releases/download/v2.3.7/$etcdVersion.zip -o $etcdVersion.zip"
cmd /C $downloadEtcd
if (-not (Test-Path "./$etcdVersion.zip")){
  Write-Host "Could not download etcd"
  exit 255
}

cmd /C "7z x $etcdVersion.zip"
cmd /C "copy $etcdVersion\etcd.exe etcd.exe"
cmd /C "copy $etcdVersion\etcdctl.exe etcdctl.exe"

#start etcd
$etcdCmd = -join("--initial-cluster-token etcd-cluster --advertise-client-urls http://",$ip,":2379 --listen-peer-urls http://",$ip,":2380 --listen-client-urls http://",$ip,":2379")
echo "Running $etcd $etcdCmd"
$etcdProcess = Start-Process "$etcd" -Args "$etcdCmd" -PassThru
If (-Not($?)){
  Write-Host "Could not start etcd server"
  exit 255
}


#set initial subnet config
$etcdSetCmd = -join ("$etcdctl --endpoints=http://",$ip,":2379 set /coreos.com/network/config '{\`"Network\`":\`"10.0.0.0/8\`", \`"SubnetLen\`":20, \`"SubnetMin\`":\`"10.10.0.0\`", \`"SubnetMax\`":\`"10.99.0.0\`", \`"Backend\`": { \`"Type\`":\`"host-gw\`"}}'")
echo "Running $etcdSetCmd :)"
iex $etcdSetCmd

If (-Not($?)){
  Write-Host "Could not set subnet values in etcd"
  exit 255
}

#start flanneld as server so that we can add new routes through curl
$fServerCmd = -join ("--etcd-endpoints=http://",$ip,":2379 --listen :8888")
echo "Running $flanneld $fServerCmd"
$flannelServerProcess = Start-Process "$flanneld" -Args "$fServerCmd" -PassThru

If (-Not($?)){
  Write-Host "Could not start flanneld server"
  exit 255
}

#start flanneld - this one will add routes to the routing table
$fCmd = -join ("--etcd-endpoints=http://"+$ip+":2379")
Write-Host "Running $flanneld $fCmd"
$flannelProcess = Start-Process "$flanneld" -Args "$fCmd" -PassThru

If (-Not($?)){
  Write-Host "Could not start flanneld"
  exit 255
}

#curl -L -H "ContentType: application/json" -X POST http://10.11.0.43:8888/v1/_/leases -d "{\"PublicIP\": \"10.12.10.10\",\"BackendType\":\"host-gw\"}"
$curlAdd = -join("curl -L -H `"ContentType: application/json`" -X POST http://",$ip,":8888/v1/_/leases -d `"{\`"PublicIP\`":\`"$fakeIP\`",\`"BackendType\`":\`"host-gw\`"}`"")
Write-Host "Running $curlAdd"
$rez = cmd /C $curlAdd

If (-Not($?)){
  Write-Host "Could not add networks to flanneld server"
  exit 255
}

$val = ConvertFrom-json $rez
$searchTerm = $val.Subnet

Write-Host "Looking for $searchTerm"

$subnet=Get-NetRoute -DestinationPrefix $val.Subnet

if(!$subnet){
  Write-Host "The subnet $subnet was not found in the routing table"
  Get-NetRoute
  exit 255
}

Write-Host "Success"

Stop-Process $flannelProcess
Stop-Process $flannelServerProcess
Stop-Process $etcdProcess
