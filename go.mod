module github.com/flannel-io/flannel

go 1.16

require (
	github.com/Microsoft/hcsshim v0.9.2
	github.com/bronze1man/goStrongswanVici v0.0.0-20201105010758-936f38b697fd
	github.com/containernetworking/plugins v0.9.1
	github.com/coreos/go-iptables v0.5.0
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/joho/godotenv v0.0.0-20161216230537-726cc8b906e3
	github.com/jonboulle/clockwork v0.2.2
	github.com/pkg/errors v0.9.1
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/vishvananda/netns v0.0.0-20200728191858-db3c7e526aae
	golang.org/x/net v0.0.0-20220907135653-1e95f45603a7
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20211230205640-daad0b7ba671
	k8s.io/api v0.23.13
	k8s.io/apimachinery v0.23.13
	k8s.io/client-go v0.23.13
	k8s.io/klog v1.0.0

)

require (
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.0.464
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc v1.0.464
	go.etcd.io/etcd/client/v3 v3.5.4
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	go.etcd.io/etcd/api/v3 v3.5.4
	go.etcd.io/etcd/client/pkg/v3 v3.5.4
	go.etcd.io/etcd/tests/v3 v3.5.4
	golang.org/x/text v0.4.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	k8s.io/klog/v2 v2.70.1 // indirect
)
