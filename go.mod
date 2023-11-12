module github.com/flannel-io/flannel

go 1.20

// replace github.com/dgrijalva/jwt-go => github.com/golang-jwt/jwt/v4 v4.2.0

require (
	github.com/Microsoft/hcsshim v0.9.10
	github.com/bronze1man/goStrongswanVici v0.0.0-20221114103242-3f6dc524986c
	github.com/containernetworking/plugins v1.3.0
	github.com/coreos/go-iptables v0.7.0
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/joho/godotenv v0.0.0-20161216230537-726cc8b906e3
	github.com/jonboulle/clockwork v0.4.0
	github.com/pkg/errors v0.9.1
	github.com/vishvananda/netlink v1.2.1-beta.2
	github.com/vishvananda/netns v0.0.4
	go.uber.org/zap v1.17.0 // indirect
	golang.org/x/net v0.17.0
	golang.org/x/oauth2 v0.11.0 // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	golang.zx2c4.com/wireguard v0.0.0-20230325221338-052af4a8072b // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230429144221-925a1e7659e6
	google.golang.org/genproto v0.0.0-20231012201019-e917dd12ba7a // indirect
	k8s.io/api v0.25.2
	k8s.io/apimachinery v0.25.2
	k8s.io/client-go v0.25.2
	k8s.io/klog/v2 v2.100.1

)

require (
	github.com/avast/retry-go/v4 v4.3.2
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.0.464
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc v1.0.464
)

require (
	github.com/PuerkitoBio/purell v1.1.1 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.19.5 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.6 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.11.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.26.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20201229170055-e5319fda7802 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.8 // indirect
	go.etcd.io/etcd/client/v2 v2.305.10 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.10 // indirect
	go.etcd.io/etcd/raft/v3 v3.5.10 // indirect
	go.etcd.io/etcd/server/v3 v3.5.10 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.46.0 // indirect
	go.opentelemetry.io/otel v1.20.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.0.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.0.1 // indirect
	go.opentelemetry.io/otel/metric v1.20.0 // indirect
	go.opentelemetry.io/otel/sdk v1.0.1 // indirect
	go.opentelemetry.io/otel/trace v1.20.0 // indirect
	go.opentelemetry.io/proto/otlp v0.9.0 // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231002182017-d307bd883b97 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
)

require (
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/containerd/cgroups v1.1.0 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.4.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.etcd.io/etcd/api/v3 v3.5.10
	go.etcd.io/etcd/client/pkg/v3 v3.5.10
	go.etcd.io/etcd/client/v3 v3.5.10
	go.etcd.io/etcd/tests/v3 v3.5.10
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/grpc v1.59.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/kube-openapi v0.0.0-20220803162953-67bda5d908f1 // indirect
	k8s.io/utils v0.0.0-20220728103510-ee6ede2d64ed
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)
