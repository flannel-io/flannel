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
2. Git clone the flannel repo. It MUST be placed in your GOPATH under `github.com/coreos/flannel`: `cd $GOPATH/src; git clone https://github.com/coreos/flannel.git`
3. Run the build script, ensuring that `CGO_ENABLED=1`: `cd flannel; CGO_ENABLED=1 make dist/flanneld` for linux usage.
   Run the build script, ensuring that `CGO_ENABLED=1`: `cd flannel; CGO_ENABLED=1 make dist/flanneld.exe` for windows usage.

# Release Process

1. Create a release on GitHub and use it to create a tag.
2. Check the tag out and run
    * `make release`
3. Attach all the files in `dist` to the GitHub release.
4. Run `make docker-push-all` to push all the images to a registry.

# Obtaining master builds

A new build of flannel is created for every commit to master. They can be obtained from [https://quay.io/repository/coreos/flannel-git](https://quay.io/repository/coreos/flannel-git?tab=tags )

* `latest` is always the current HEAD of master. Use with caution
* The image tags have a number of components e.g. `v0.7.0-109-gb366263c-amd64`
  * The last release was `v0.7.0`
  * This version is 109 commits newer
  * The commit hash is `gb366263c`
  * The platform is `amd64`

These builds can be useful when a particular commit is needed for a specific feature or bugfix.

NOTE: the image name is `quay.io/coreos/flannel-git` for master builds. *Releases* are named `quay.io/coreos/flannel` (there is no `-git` suffix).
