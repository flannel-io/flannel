## Building flannel
The most reliable way to build flannel is by using Docker.
### Building in a Docker container
To build flannel in a container run `make dist/flanneld-amd64`
You will now have a `flanneld-amd64` binary in the `dist` directory.

### Building manually
* Step 1: Make sure you have required dependencies installed on your machine.
** On Ubuntu, run `sudo apt-get install linux-libc-dev golang gcc`.
** On Fedora/Redhat, run `sudo yum install kernel-headers golang gcc`.
* Step 2: Git clone the flannel repo. It MUST be placed in your GOPATH under `github.com/coreos/flannel`: `cd $GOPATH/src; git clone https://github.com/coreos/flannel.git`
* Step 3: Run the build script, ensuring that `CGO_ENABLED=1`: `cd flannel; CGO_ENABLED=1 make dist/flanneld`

## Release Process
* Create a release on Github and use it to create a tag
* Check the tag out and run
  * `make release`
* Attach all the files in `dist` to the Github release
* Run `make docker-push-all` to push all the images to a registry