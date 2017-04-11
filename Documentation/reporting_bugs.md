# Reporting bugs

If any part of the flannel project has bugs or documentation mistakes, please let us know by [opening an issue][flannel-issue]. Before creating a bug report, please check that an issue reporting the same problem does not already exist.

To make the bug report accurate and easy to understand, please try to create bug reports that are:

- Specific. Include as much details as possible: which version, what environment, what configuration, etc.

- Reproducible. Include the steps to reproduce the problem. We understand some issues might be hard to reproduce, please includes the steps that might lead to the problem.

- Isolated. Please try to isolate and reproduce the bug with minimum dependencies. It would significantly slow down the speed to fix a bug if too many dependencies are involved in a bug report. Debugging external systems that rely on flannel is out of scope, but we are happy to provide guidance in the right direction or help with using flannel itself.

- Unique. Do not duplicate an existing bug report.

- Scoped. One bug per report. Do not follow up with another bug inside one report.

It may be worthwhile to read [Elika Etemad’s article on filing good bug reports][filing-good-bugs] before creating a bug report.

We might ask for further information to locate a bug. A duplicated bug report will be closed.

## Frequently asked questions

### How to get a stack trace

``` bash
$ kill -QUIT $PID
```

### How to get flannel version

``` bash
$ flannel --version
```

[flannel-issue]: https://github.com/coreos/flannel/issues/new
[filing-good-bugs]: http://fantasai.inkedblade.net/style/talks/filing-good-bugs/
