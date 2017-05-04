# Building flannel

The most reliable way to build flannel is by using Docker.

## Building in a Docker container

To build flannel in a container run `make dist/flanneld-amd64`.
You will now have a `flanneld-amd64` binary in the `dist` directory.

## Building manually

1. Make sure you have required dependencies installed on your machine.
    * On Ubuntu, run `sudo apt-get install linux-libc-dev golang gcc`.
    * On Fedora/Redhat, run `sudo yum install kernel-headers golang gcc`.
2. Git clone the flannel repo. It MUST be placed in your GOPATH under `github.com/coreos/flannel`: `cd $GOPATH/src; git clone https://github.com/coreos/flannel.git`
3. Run the build script, ensuring that `CGO_ENABLED=1`: `cd flannel; CGO_ENABLED=1 make dist/flanneld`

# Release Process

1. Create a release on GitHub and use it to create a tag.
2. Check the tag out and run
    * `make release`
3. Attach all the files in `dist` to the GitHub release.
4. Run `make docker-push-all` to push all the images to a registry.
