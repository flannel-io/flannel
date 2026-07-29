# Fuzzing flannel

flannel is continuously fuzzed with [ClusterFuzzLite](https://google.github.io/clusterfuzzlite/),
which runs [Go native fuzz tests](https://go.dev/doc/security/fuzz/) from GitHub Actions.

Fuzzing exercises the code paths that decode untrusted input — mainly the network
configuration JSON and the values read back from the datastore (etcd / the Kubernetes
API) — to find panics, hangs and other crashes on malformed data.

## Fuzz targets

The fuzz targets live next to the code they exercise, in standard `*_test.go` files:

| Target | Package | Function under test |
| --- | --- | --- |
| `FuzzParseConfig` | `pkg/subnet` | `ParseConfig` (network config JSON) |
| `FuzzParseSubnetKey` | `pkg/subnet` | `ParseSubnetKey` (subnet keys like `10.5.1.0-24`) |
| `FuzzNodeToLease` | `pkg/subnet/kube` | `nodeToLease` (Node annotations + PodCIDR from the k8s API) |
| `FuzzKvToIPLease` | `pkg/subnet/etcd` | `kvToIPLease` (etcd subnet key + LeaseAttrs JSON) |
| `FuzzParseIP4` / `FuzzParseIP6` | `pkg/ip` | `ParseIP4` / `ParseIP6` |
| `FuzzIP4UnmarshalJSON` / `FuzzIP6UnmarshalJSON` | `pkg/ip` | `IP4.UnmarshalJSON` / `IP6.UnmarshalJSON` |
| `FuzzIP4NetUnmarshalJSON` / `FuzzIP6NetUnmarshalJSON` | `pkg/ip` | `IP4Net.UnmarshalJSON` / `IP6Net.UnmarshalJSON` |
| `FuzzParseVXLANConfig` | `pkg/backend/vxlan` | `parseVXLANConfig` (VXLAN backend config JSON) |

All targets are side-effect free: they do not touch netlink, iptables/nftables or the
network, so they run safely without special privileges.

## Running fuzzers locally

Because flannel is a Linux project, run the fuzzers on Linux (natively or in a
`golang` container). Use the standard `go test -fuzz` flag, targeting one function at a
time:

```bash
# Fuzz the config parser for 30 seconds.
go test -run '^$' -fuzz '^FuzzParseConfig$' -fuzztime 30s ./pkg/subnet/

# Fuzz IPv4 parsing.
go test -run '^$' -fuzz '^FuzzParseIP4$' -fuzztime 30s ./pkg/ip/

# Fuzz the VXLAN backend config parser.
go test -run '^$' -fuzz '^FuzzParseVXLANConfig$' -fuzztime 30s ./pkg/backend/vxlan/
```

Any crash is written to `testdata/fuzz/<FuzzName>/` inside the package. Commit that file
to turn the reproducer into a permanent regression seed — it will then run as part of the
normal `go test` suite.

## Continuous fuzzing (CI)

ClusterFuzzLite is wired up through the build integration in `.clusterfuzzlite/`
(`Dockerfile`, `build.sh`, `project.yaml`) and the following workflows:

- `.github/workflows/cflite_pr.yml` — fuzzes changed code on every pull request
  (`code-change` mode, ~300s per sanitizer).
- `.github/workflows/cflite_batch.yml` — daily longer batch fuzzing run that grows the
  shared corpus.
- `.github/workflows/cflite_cron.yml` — daily corpus pruning and coverage report.
- `.github/workflows/cflite_build.yml` — builds and uploads the fuzzers on every push to
  `master` so PR fuzzing can tell whether a crash is newly introduced.

The corpus and builds are persisted using the default GitHub Actions cache/artifact
storage. To share a corpus across forks or keep it longer, configure a dedicated
[storage repo](https://google.github.io/clusterfuzzlite/running-clusterfuzzlite/github-actions/#storage-repo)
via the `storage-repo` inputs.

## Adding a new fuzz target

1. Add a `FuzzXxx(f *testing.F)` function in a `*_test.go` file in the package you want to
   fuzz. Seed it with a few `f.Add(...)` inputs and keep it free of side effects.
2. Register it in `.clusterfuzzlite/build.sh` by adding a line to the `targets` array:
   `"./pkg/your/pkg::FuzzXxx::fuzz_xxx"`.
3. Run it locally with `go test -fuzz` to make sure it builds and finds no immediate crash.
