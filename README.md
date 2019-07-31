# Go DX

Welcome to the official Go implementation of [DxChain](https://www.dxchain.com) protocol!

[![CircleCI](https://circleci.com/gh/DxChainNetwork/godx.svg?style=svg&circle-token=f2062f8bae0aee80ef408bcfff103e2ab73d8b39)](https://circleci.com/gh/DxChainNetwork/godx) 
[![Golang](https://img.shields.io/badge/go-1.11.4-blue.svg)](https://golang.org/dl/)
[![release](https://img.shields.io/badge/release-v0.8.0-blue)](https://github.com/DxChainNetwork/godx/releases)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
![Open Source Love](https://badges.frapsoft.com/os/v1/open-source.png?v=103)



`gdx` program is built on top of the DxChain protocol. DxChain is a blockchain based P2P network for data storage. The core feature is that user can upload data to the network as storage client or provide data storage service for other peers in the network as a storage host. In addition, DxChain also contains features that are supported by other blockchains, such as distributed ledger and smart contracts.


**Table of contents:**
- [Go DX](#go-dx)
- [Getting Started](#getting-started)
  - [1. Prerequisites](#1-prerequisites)
    - [1.1. Golang](#11-golang)
    - [1.2. Xcode](#12-xcode)
    - [1.3. Go Vendor](#13-go-vendor)
    - [1.4. Docker](#14-docker)
  - [2. Build from source](#2-build-from-source)
    - [2.1. Clone Project](#21-clone-project)
    - [2.2. Packages Installation](#22-packages-installation)
    - [2.3. Build](#23-build)
- [Running `gdx`](#running-gdx)
  - [1. Run as miner](#1-run-as-miner)
  - [2. Run as storage client](#2-run-as-storage-client)
  - [3. Run as storage host](#3-run-as-storage-host)
- [Basic Console Commands](#basic-console-commands)
  - [1. Account](#1-account)
  - [2. Mining](#2-mining)
  - [3. Chain](#3-chain)
  - [4. StorageClient](#4-storageclient)
  - [5. StorageHost](#5-storagehost)
- [Tutorial](#tutorial)
- [Contact](#contact)

# Getting Started

## 1. Prerequisites

### 1.1. Golang

To build the program from the source code, Golang 1.11 is required. Please follow [this link](https://golang.org/doc/install) to install Golang on your computer. To check if the program is installed successfully, use the following command:

```shell
$ go version
```

### 1.2. Xcode

Xcode can be installed from the App Store

### 1.3. Go Vendor

[Go Vendor](https://github.com/kardianos/govendor) is a package management tool used for this project. It can be installed via: 

```shell
$ go get -u -v github.com/kardianos/govendor
```

### 1.4. Docker

The installation of Docker is optional. Docker is used for cross-platform build meaning if you want to build linux version of `gdx` on your MacBook, you have to install Docker. Docker can be installed via: 

```shell
$ brew cask install docker
```

## 2. Build from source

### 2.1. Clone Project

```shell
$ mkdir -p $GOPATH/src/github.com/DxChainNetwork
$ cd $GOPATH/src/github.com/DxChainNetwork
$ git clone git@github.com:DxChainNetwork/go-dxc.git
```

### 2.2. Packages Installation

Required packages can be installed via go vendor

```shell
cd go-dxc
govendor sync -v
```

All packages saved in the `gdx/vendor/vendor.json` will be downloaded. It may take some time.

### 2.3. Build

**Build for your operating system**

```shell
$ make gdx
```

**Cross platform build**

```shell
$ make gdx-cross
```

# Running `gdx`

A node in the DxChain Network can become the following three roles
* Storage Client
* Storage Host
* Miner

**NOTE:** each node can become a miner regardless of its role

To run the node that is capable of performing all operations, use the following command:

```shell
$ gdx
```

## 1. Run as miner

If you do not intend to become neither a storage client nor a storage host, you can start the program by running the following command in the terminal

```shell
$ gdx --role miner
```

## 2. Run as storage client

By paying DX tokens to storage hosts, storage client is able to rent storage space and store files in the DX network safely and securely. When needed, storage client can download those files from the network.

If you intend to become a storage client only, run the following command in the terminal

```shell
$ gdx --role client
```

## 3. Run as storage host

Storage host serves as a storage service provider, gaining profit for storing data uploaded by the storage client.

If you intend to become a storage host only, run the following command in the terminal

```shell
$ gdx --role host
```

# Basic Console Commands

Once the `gdx` program is up and running, you can use the following command to open the `gdx` console to perform different operations, such as mining, creating storage contract, uploading files, and etc.

```shell
$ gdx attach
```

## 1. Account

| Command                                                                   | Description                                                                                                                                                                                                                                                              |
|---------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| personal.newAccount()                                                     | Generate a new account                                                                                                                                                                                                                                                   |
| eth.accounts                                                              | Retrieve all accounts                                                                                                                                                                                                                                                    |
| personal.unlockAccount(eth.accounts[*ACCOUNT_INDEX*], "*YOUR_PASSWORD*", 0) | Permanently unlock the account of your choice. This is generally used for storage related operations because most storage related operations, such as upload, download, contract create, and contract renew, requires unlocked account, so money can be withdraw from it |
| personal.unlockAccount(eth.accounts[*ACCOUNT_INDEX*])                     | Unlock the account of your choice. The account will be automatically locked after a while                                                                                                                                                                                |
| eth.getBalance(eth.accounts[*ACCOUNT_INDEX*])                             | Check the account balance                                                                                                                                                                                                                                                |

## 2. Mining

| Command       | Description                 |
|---------------|-----------------------------|
| miner.start() | Start mining                |
| miner.stop()  | Stop Mining                 |
| eth.mining()  | Check if the node is mining |

## 3. Chain

| Command         | Description                                        |
|-----------------|----------------------------------------------------|
| eth.blockHeight | Current block height                               |
| admin.peers     | Check number of nodes connected to the local nodes |
| admin.nodeInfo  | Check the local node's information                 |

## 4. StorageClient



## 5. StorageHost



# Tutorial

The basic tutorial can be found [here]()

# Contact

Thank you so much for your support and your confidence in this project. If you have any question, please do not hesitated to contact us via support@dxchain.com
