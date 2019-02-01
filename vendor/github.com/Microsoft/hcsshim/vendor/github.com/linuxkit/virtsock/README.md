
This repository contains Go bindings and sample code for [Hyper-V sockets](https://msdn.microsoft.com/en-us/virtualization/hyperv_on_windows/develop/make_mgmt_service) and [virtio sockets](http://stefanha.github.io/virtio/)(VSOCK).

## Organisation

- `pkg/hvsock`: Go binding for Hyper-V sockets
- `pkg/vsock`: Go binding for virtio VSOCK
- `cmd/sock_stress`: A stress test program for virtsock
- `cmd/vsudd`: A unix domain socket to virtsock proxy (used in Docker for Mac/Windows)
- `scripts`: Miscellaneous scripts
- `c`: Sample C code (including benchmarks and stress tests)
- `data`: Data from benchmarks


## Building

By default the Go sample code is build in a container. Simply type `make`.

If you want to build binaries on a local system use `make build-binaries`.

## Testing

There are several examples and tests written both in [Go](./cmd) and in [C](./c). The C code is Hyper-V sockets specific while the Go code also works with virtio sockets and [HyperKit](https://github.com/moby/hyperkit). The respective READMEs contain instructions on how to run the tests, but the simplest way is to use [LinuxKit](https://github.com/linuxkit/linuxkit).

Assuming you have LinuxKit installed, the make target `make linuxkit`
will build a custom Linux image which can be booted on HyperKit or on
Windows. The custom Linux image contains the test binaries.

### macOS

Boot the Linux VM:
```
linuxkit run hvtest
```
This should create a directory called `./hvtest-state`.

Run the server in the VM and client on the host:
```
linux$ sock_stress -s vsock -v 1
macos$ ./bin/sock_stress.darwin -c vsock://3 -m hyperkit:./hvtest-state -v 1
```

Run the server on the host and the client inside the VM:
```
macos$ ./bin/sock_stress.darwin -s vsock -m hyperkit:./hvtest-state -v 1
linux$ sock_stress -c vsock://2 -v 1
```

### Windows

On Windows we currently only support the server to be run inside the
VM and the host connecting to it. In the future we will support
running the server on the host as well.

For Linux guests on Windows there are two different implmentations,
one in the LinuxKit 4.9.x kernels and one in 4.14.x upstream
kernels. They require different protocols to be used. The
`sock_stress` and `vsudd` programs automatically detect which version
to use.

Boot the Linux VM (from an elevated powershell):
```
linuxkit run -name hvtest hvtest-efi.iso
```

Run the server in the VM and client on the host:
```
linux$ sock_stress -v 1 -s hvsock
win$ sock_stress -v 1 -c hvsock://<VM ID>
```
(where `<VM ID>` is from the output of: `(get-vm hvtest).Id`)

Run the server on the host and the client inside the VM:
```
win$ sock_stress -v 1 -s hvsock
linux$ sock_stress -v 1 -c hvsock://parent
```
**Note:** This may fail on the client with receiving unexpected EOFs (see below).


## Known limitations

- `hvsock`: When running the server on the host with a client in a
  Linux VM, it looks like unidirectional `shutdown()` is not working
  properly. There appears to be a race of sort.

- `hvsock`: Hyper-V socket implementations prior to Windows build
  10586 (aka 1511, aka Threshold 2) was buggy. There may even be
  issues with build prior to build 14393 (aka 1607, aka Redstone 1).
  
- `hvsock`: Earlier versions of this code supported the older Windows
  builds, but support has now been removed. If you require the older version,
  please use the `end_10586_tag`.

- `vsock`: There is general host side implementation as the interface
  is hypervisor specific. The `vsock` package includes some support
  for connecting with the VSOCK implementation in
  [Hyperkit](https://github.com/moby/hyperkit), but there is no
  implementation for, e.g. `qemu`.

