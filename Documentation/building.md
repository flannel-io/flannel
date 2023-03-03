# Building flannel

The most reliable way to build flannel is by using Docker.

## Building in a Docker container

To build flannel in a container run `make dist/flanneld-amd64`.
You will now have a `flanneld-amd64` binary in the `dist` directory.

## Building for other platforms

If you're not running `amd64` then you need to manually set `ARCH` before running `make`. For example, to produce a 
`flanneld-s390x` binary and image, run
* ARCH=s390x make image

If you want to cross-compile for a different platform (e.g. you're running `amd64` but you want to produce `arm` binaries) then you need the qemu-static binaries to be present in `/usr/bin`. They can be installed on Ubuntu with
* `sudo apt-get install qemu-user-static`

Then you should be able to set the ARCH as above
* ARCH=arm make image

## Building manually

1. Make sure you have required dependencies installed on your machine.
    * On Ubuntu, run `sudo apt-get install linux-libc-dev golang gcc`. 
      If the golang version installed is not 1.7 or higher. Download the newest golang and install manully.
      To build the flannel.exe on windows, mingw-w64 is also needed. Run command `sudo apt-get install mingw-w64`
    * On Fedora/Redhat, run `sudo yum install kernel-headers golang gcc glibc-static`.
2. Git clone the flannel repo. It MUST be placed in your GOPATH under `github.com/flannel-io/flannel`: `cd $GOPATH/src; git clone https://github.com/flannel-io/flannel.git`
3. Run the build script, ensuring that `CGO_ENABLED=1`: `cd flannel; CGO_ENABLED=1 make dist/flanneld` for linux usage.
   Run the build script, ensuring that `CGO_ENABLED=1`: `cd flannel; CGO_ENABLED=1 make dist/flanneld.exe` for windows usage.

