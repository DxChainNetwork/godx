# Linux and MacOS Setup

## 1. Prerequisites

### 1.1. Golang

To build the program from the source code, Golang 1.11 is required. Please follow [this link](https://golang.org/doc/install) to install Golang on your computer. To check if the program is installed successfully, use the following command:

```shell
$ go version
```

### 1.2. Xcode (for macOS only)

Xcode can be installed from the App Store

### 1.3. Go Vendor

[Go Vendor](https://github.com/kardianos/govendor) is a package management tool used for this project. It can be installed via: 

```shell
$ go get -u -v github.com/kardianos/govendor
```

### 1.4. Docker (Optional)

The installation of Docker is optional. Docker is used for cross-platform build meaning if you want to build linux version of `gdx` on your MacBook, you have to install Docker. Please follow the following installation guide:

[Docker Installation Guide for Mac](https://docs.docker.com/docker-for-mac/install/)

[Docker Installation Guide for Linux](https://runnable.com/docker/install-docker-on-linux)

## 2. Build from source

### 2.1. Clone Project

```shell
$ mkdir -p $GOPATH/src/github.com/DxChainNetwork
$ cd $GOPATH/src/github.com/DxChainNetwork
$ git clone git@github.com:DxChainNetwork/godx.git
```

**NOTE:** please checkout to the latest release branch by using the following command:

```shell
$ git checkout release0.8.0
```

### 2.2. Packages Installation

Required packages can be installed via go vendor,

```shell
$ cd $GOPATH/src/github.com/DxChainNetwork/godx
$ govendor sync -v
```

All packages saved in the `godx/vendor/vendor.json` will be downloaded. Please wait for download to finish. 

### 2.3. Build

**Build for your operating system**

```shell
$ make gdx
```

### 2.4. Add `gdx` to path

Add `gdx` executable to your path by going through the following commands:

```shell
$ cd $GOPATH/src/github.com/DxChainNetwork/godx/build/bin
$ export PATH=$PATH:$(pwd)
```

For each terminal you open, you have to run the above commands. For more advanced users, you can add the export statement in the shell init file like `~/.bash_profile` or `~/.zshrc`