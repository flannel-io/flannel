module github.com/coreos/flannel

go 1.13

replace github.com/benmoss/go-powershell => github.com/k3s-io/go-powershell v0.0.0-20201118222746-51f4c451fbd7

require (
	github.com/Microsoft/go-winio v0.4.11 // indirect
	github.com/Microsoft/hcsshim v0.8.6-0.20190129145542-bc49f75c7221
	github.com/aws/aws-sdk-go v1.12.54
	github.com/benmoss/go-powershell v0.0.0-00010101000000-000000000000 // indirect
	github.com/bronze1man/goStrongswanVici v0.0.0-20171013065002-4d72634a2f11
	github.com/coreos/etcd v3.1.11+incompatible
	github.com/coreos/go-iptables v0.4.0
	github.com/coreos/go-systemd v0.0.0-20161114122254-48702e0da86b
	github.com/coreos/pkg v0.0.0-20160727233714-3ac0863d7acf
	github.com/denverdino/aliyungo v0.0.0-20170629053852-f6cab0c35083
	github.com/go-ini/ini v1.28.1 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20160803190731-bd40a432e4c7 // indirect
	github.com/joho/godotenv v0.0.0-20161216230537-726cc8b906e3
	github.com/jonboulle/clockwork v0.1.0
	github.com/k3s-io/go-powershell v0.0.0-20200701182037-6845e6fcfa79 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1
	github.com/rakelkar/gonetsh v0.0.0-20190930180311-e5c5ffe4bdf0
	github.com/sirupsen/logrus v1.0.6 // indirect
	github.com/smartystreets/goconvey v1.6.4 // indirect
	github.com/stretchr/testify v1.6.1 // indirect
	github.com/ugorji/go v0.0.0-20170107133203-ded73eae5db7 // indirect
	github.com/vishvananda/netlink v0.0.0-20170220200719-fe3b5664d23a
	github.com/vishvananda/netns v0.0.0-20170219233438-54f0e4339ce7
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	google.golang.org/api v0.4.0
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	k8s.io/api v0.18.5
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v0.18.5
	k8s.io/klog v1.0.0
	k8s.io/utils v0.0.0-20200324210504-a9aa75ae1b89
)
