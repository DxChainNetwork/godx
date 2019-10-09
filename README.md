# Go DX

Welcome to the official Go implementation of [DxChain](https://www.dxchain.com) protocol!

[![CircleCI](https://circleci.com/gh/DxChainNetwork/godx.svg?style=svg&circle-token=f2062f8bae0aee80ef408bcfff103e2ab73d8b39)](https://circleci.com/gh/DxChainNetwork/godx)
[![Go Report Card](https://goreportcard.com/badge/github.com/DxChainNetwork/godx)](https://goreportcard.com/report/github.com/DxChainNetwork/godx)
[![Coverage](https://codecov.io/gh/DxChainNetwork/godx/branch/master/graph/badge.svg)](https://codecov.io/gh/DxChainNetwork/godx)
[![Golang](https://img.shields.io/badge/go-1.11.4-blue.svg)](https://golang.org/dl/)
[![release](https://img.shields.io/badge/release-v0.9.0-blue)](https://github.com/DxChainNetwork/godx/releases)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Open Source Love](https://badges.frapsoft.com/os/v1/open-source.png?v=103)](https://opensource.org/)

`gdx` program is built on top of the DxChain protocol. DxChain is a blockchain based P2P network for data storage. The core feature is that user can upload data to the network as storage client or provide data storage service for other peers in the network as a storage host. In addition, DxChain also contains features that are supported by other blockchain, such as distributed ledger and smart contracts.

**NOTE: the `master` branch will always contain the most active code. However, it is not stable.**

**Table of contents:**
- [Go DX](#go-dx)
- [Section 1. Getting Started](#section-1-getting-started)
  - [1.1. Prerequisites](#11-prerequisites)
    - [1.1.1. Golang](#111-golang)
    - [1.1.2. Xcode (for macOS only)](#112-xcode-for-macos-only)
    - [1.1.3. Go Vendor](#113-go-vendor)
    - [1.1.4. Docker (Optional)](#114-docker-optional)
  - [1.2. Build from source](#12-build-from-source)
    - [1.2.1. Clone Project](#121-clone-project)
    - [1.2.2. Packages Installation](#122-packages-installation)
    - [1.2.3. Build](#123-build)
    - [1.2.4. Add `gdx` to path](#124-add-gdx-to-path)
- [Section 2. Running `gdx`](#section-2-running-gdx)
  - [2.1. Run as miner](#21-run-as-miner)
  - [2.2. Run as storage client](#22-run-as-storage-client)
  - [2.3. Run as storage host](#23-run-as-storage-host)
- [Section 3. Tutorial](#section-3-tutorial)
- [Section 4. Basic Console Commands](#section-4-basic-console-commands)
  - [4.1. Account](#41-account)
    - [4.1.1. personal.newAccount](#411-personalnewaccount)
    - [4.1.2. eth.accounts](#412-ethaccounts)
    - [4.1.3. personal.unlockAccount](#413-personalunlockaccount)
    - [4.1.4. eth.getBalance](#414-ethgetbalance)
  - [4.2. Mining](#42-mining)
    - [4.2.1. miner.start](#421-minerstart)
    - [4.2.2. miner.stop](#422-minerstop)
    - [4.2.3. eth.mining](#423-ethmining)
  - [4.3. Node](#43-node)
    - [4.3.1. eth.blockHeight](#431-ethblockheight)
    - [4.3.2. admin.peers](#432-adminpeers)
    - [4.3.3. admin.nodeInfo](#433-adminnodeinfo)
  - [4.4. StorageClient](#44-storageclient)
    - [4.4.1. sclient.host.<span>ls](#441-sclienthostspanls)
    - [4.4.2. sclient.paymentAddr](#442-sclientpaymentaddr)
    - [4.4.3. sclient.setPaymentAddr](#443-sclientsetpaymentaddr)
    - [4.4.4. sclient.setConfig](#444-sclientsetconfig)
    - [4.4.5. sclient.config](#445-sclientconfig)
    - [4.4.6. sclient.contracts](#446-sclientcontracts)
    - [4.4.7 sclient.contract](#447-sclientcontract)
    - [4.4.8 sclient.upload](#448-sclientupload)
    - [4.4.9 sclient.download](#449-sclientdownload)
    - [4.4.10 sclient.file.<span>ls](#4410-sclientfilespanls)
    - [4.4.11 sclient.file.rename](#4411-sclientfilerename)
    - [4.4.12 sclient.file.delete](#4412-sclientfiledelete)
  - [4.5. StorageHost](#45-storagehost)
    - [4.5.1. shost.config](#451-shostconfig)
    - [4.5.2. shost.setConfig](#452-shostsetconfig)
    - [4.5.3. shost.paymentAddr](#453-shostpaymentaddr)
    - [4.5.4. shost.announce](#454-shostannounce)
    - [4.5.5. shost.folder.add](#455-shostfolderadd)
    - [4.5.6. shost.folder.<span>ls](#456-shostfolderspanls)
    - [4.5.7. shost.folder.resize](#457-shostfolderresize)
    - [4.5.8. shost.folder.delete](#458-shostfolderdelete)
- [License](#license)
- [Appendix](#appendix)
  - [Units](#units)
  - [Templates](#templates)
- [Contact](#contact)

# Section 1. Getting Started

## 1.1. Prerequisites

**NOTE:** currently, we only support MacOS and Linux. Windows is not supported yet.

### 1.1.1. Golang

To build the program from the source code, Golang 1.11 is required. Please follow [this link](https://golang.org/doc/install) to install Golang on your computer. To check if the program is installed successfully, use the following command:

```shell
$ go version
```

### 1.1.2. Xcode (for macOS only)

Xcode can be installed from the App Store

### 1.1.3. Go Vendor

[Go Vendor](https://github.com/kardianos/govendor) is a package management tool used for this project. It can be installed via: 

```shell
$ go get -u -v github.com/kardianos/govendor
```

### 1.1.4. Docker (Optional)

The installation of Docker is optional. Docker is used for cross-platform build meaning if you want to build linux version of `gdx` on your MacBook, you have to install Docker. Please follow the following installation guide:

[Docker Installation Guide for Mac](https://docs.docker.com/docker-for-mac/install/)

[Docker Installation Guide for Linux](https://runnable.com/docker/install-docker-on-linux)

## 1.2. Build from source

### 1.2.1. Clone Project

```shell
$ mkdir -p $GOPATH/src/github.com/DxChainNetwork
$ cd $GOPATH/src/github.com/DxChainNetwork
$ git clone git@github.com:DxChainNetwork/godx.git
```

**NOTE:** please checkout to the latest release branch by using the following command:

```shell
$ git checkout release0.8.0
```

### 1.2.2. Packages Installation

Required packages can be installed via go vendor,

```shell
$ cd $GOPATH/src/github.com/DxChainNetwork/godx
$ govendor sync -v
```

All packages saved in the `godx/vendor/vendor.json` will be downloaded. Please wait for download to finish. 

### 1.2.3. Build

**Build for your operating system**

```shell
$ make gdx
```

### 1.2.4. Add `gdx` to path

Add `gdx` executable to your path by going through the following commands:

```shell
$ cd $GOPATH/src/github.com/DxChainNetwork/godx/build/bin
$ export PATH=$PATH:$(pwd)
```

For each terminal you open, you have to run the above commands. For more advanced users, you can add the export statement in the shell init file like `~/.bash_profile` or `~/.zshrc`

# Section 2. Running `gdx`

In the DxChain Network, a node will always be able to perform mining operation regardless of the role the node choose to be. There are three roles available and each node can choose to become all of them at the same time or one of them only:
* Storage Client
* Storage Host
* Miner

To run the node that is capable of performing all operations, use the following command:

```shell
$ gdx
```

## 2.1. Run as miner

If you do not intend to become neither a storage client nor a storage host, you can start the program by running the following command in the terminal

```shell
$ gdx --role miner
```

## 2.2. Run as storage client

By paying DX tokens to storage hosts, storage client is able to rent storage space and store files in the DX network safely and securely. When needed, storage client can download those files from the network.

If you intend to become a storage client only, run the following command in the terminal

```shell
$ gdx --role storageclient
```

## 2.3. Run as storage host

Storage host serves as a storage service provider, gaining profit for storing data uploaded by the storage client.

If you intend to become a storage host only, run the following command in the terminal

```shell
$ gdx --role storagehost
```

# Section 3. Tutorial

> Before looking through specific tutorials, please following the [preparation instructions](https://github.com/DxChainNetwork/godx-doc/blob/master/gdx/gdx-manual/manual_en.md) first

For the storage tutorial, please click [here](https://github.com/DxChainNetwork/godx-doc/blob/master/gdx/gdx-manual/storage_manual/storage_en.md)

For the DPoS tutorial, please click [here](https://github.com/DxChainNetwork/godx-doc/blob/master/gdx/gdx-manual/dpos_manual/dpos_en.md)

# License

GoDx is released under the [Apache 2.0 License](https://opensource.org/licenses/Apache-2.0). See LICENSE for more information.

# Appendix

## Units

Duration:

| Duration/Time | Representation |
|---------------|----------------|
| b             | block          |
| h             | hour           |
| d             | day            |
| w             | week           |
| m             | month          |
| y             | year           |

Currency:

| Currency | Transfer Rate          |
|----------|------------------------|
| camel    | smallest currency unit |
| Gcamel   | 1e9 camel              |
| DX       | 1e18 camel             |

Storage Size:

| Storage Size | Representation |
|--------------|----------------|
| kb           | 1e3 bytes      |
| mb           | 1e6 bytes      |
| gb           | 1e9 bytes      |
| tb           | 1e12 bytes     |
| kib          | 1 << 10 bytes  |
| mib          | 1 << 20 bytes  |
| gib          | 1 << 30 bytes  |
| tib          | 1 << 40 bytes  |

## Templates

* To form a bug report, [Bug Report Template](./.github/ISSUE_TEMPLATE/bug_report.md) must be followed
* To request a new feature, [Feature Request Template](./.github/ISSUE_TEMPLATE/feature_request.md) must be followed
* To submit a pull request, [Pull Request Template](./.github/PULL_REQUEST_TEMPLATE/pull_request_template.md) must be followed

Contribution is welcome, see [Contributing](./CONTRIBUTING.md) for more details

# Contact

Thank you so much for your support and your confidence in this project. If you have any question, please do not hesitated to contact us via support@dxchain.com