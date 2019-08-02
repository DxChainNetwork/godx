# Go DX

Welcome to the official Go implementation of [DxChain](https://www.dxchain.com) protocol!

[![CircleCI](https://circleci.com/gh/DxChainNetwork/godx.svg?style=svg&circle-token=f2062f8bae0aee80ef408bcfff103e2ab73d8b39)](https://circleci.com/gh/DxChainNetwork/godx) 
[![Golang](https://img.shields.io/badge/go-1.11.4-blue.svg)](https://golang.org/dl/)
[![release](https://img.shields.io/badge/release-v0.8.0-blue)](https://github.com/DxChainNetwork/godx/releases)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Open Source Love](https://badges.frapsoft.com/os/v1/open-source.png?v=103)](https://opensource.org/)



`gdx` program is built on top of the DxChain protocol. DxChain is a blockchain based P2P network for data storage. The core feature is that user can upload data to the network as storage client or provide data storage service for other peers in the network as a storage host. In addition, DxChain also contains features that are supported by other blockchain, such as distributed ledger and smart contracts.


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
- [Section 5. License](#section-5-license)
- [Section 6. Appendix](#section-6-appendix)
  - [Section 6.1 Units](#section-61-units)
- [Section 7. Contact](#section-7-contact)

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

A node in the DxChain Network can become the following three roles
* Storage Client
* Storage Host
* Miner

**NOTE:** each node can become a miner regardless of its role

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
$ gdx --role client
```

## 2.3. Run as storage host

Storage host serves as a storage service provider, gaining profit for storing data uploaded by the storage client.

If you intend to become a storage host only, run the following command in the terminal

```shell
$ gdx --role host
```

# Section 3. Tutorial

The basic tutorial can be found [here](./TUTORIAL.md)


# Section 4. Basic Console Commands

Once the `gdx` program is up and running, you can use the following command to open the `gdx` console to perform different operations, such as mining, creating storage contract, uploading files, and etc.

```shell
$ gdx attach
```

## 4.1. Account

### 4.1.1. personal.newAccount

| Usage                 | Description            |
|-----------------------|------------------------|
| personal.newAccount() | Generate a new account |


Example:

```shell
> personal.newAccount()
Passphrase:
Repeat passphrase:
"0x1ec76840382bfde5ec6a03dadd71947d842577e3"
```

### 4.1.2. eth.accounts

| Usage        | Description           |
|--------------|-----------------------|
| eth.accounts | Retrieve all accounts |

Example
  
  ```shell
  > eth.accounts
  ["0x1ec76840382bfde5ec6a03dadd71947d842577e3"]
  ```

### 4.1.3. personal.unlockAccount

| Usage                                                       | Description        |
|-------------------------------------------------------------|--------------------|
| personal.unlockAccount(*address*, *passphrase*, *duration*) | Unlock the account |

**NOTE:** To send transactions, the account must be unlocked. To use storage services, the account must be unlocked permanently because the transactions are made automatically. 

The duration is measured in terms of seconds with 300 seconds as its default value. To make an account be permanently unlocked, set the duration to 0.

Example:

```shell
> personal.unlockAccount(eth.accounts[0], "", 0)
true
```
which is equivalent to

```shell
> personal.unlockAccount("0x1ec76840382bfde5ec6a03dadd71947d842577e3", "", 0)
true
```

### 4.1.4. eth.getBalance

| Usage                     | Description               |
|---------------------------|---------------------------|
| eth.getBalance(*address*) | Check the account balance |

Example:

```shell
> eth.getBalance(eth.accounts[0])
0
```

## 4.2. Mining

### 4.2.1. miner.start

| Usage         | Description  |
|---------------|--------------|
| miner.start() | Start mining |

Example:

```shell
> miner.start()
null
```

### 4.2.2. miner.stop

| Usage        | Description |
|--------------|-------------|
| miner.stop() | Stop mining |

Example:

```shell
> miner.stop()
null
```

### 4.2.3. eth.mining

| Usage      | Description                 |
|------------|-----------------------------|
| eth.mining | Check if the node is mining |

Example:

```shell
> eth.mining
false
```

## 4.3. Node

### 4.3.1. eth.blockHeight

| Usage           | Description          |
|-----------------|----------------------|
| eth.blockNumber | Current block height |


Example:

```shell
> eth.blockNumber
12
```

### 4.3.2. admin.peers

| Usage       | Description               |
|-------------|---------------------------|
| admin.peers | Number of nodes connected |

Example:

```shell
> admin.peers
[{
    caps: ["eth/63", "eth/64"],
    enode: "enode://fdac7145904eba296b91b21dbc2fb75c114ab3a4e2cf5076ea95ae278e142d9b40da8b7c061beb179523781b8d4207de69266862090ac7b24e3afa62e5bb9629@35.164.203.139:36000",
    id: "578c262796d0c566e968cef804e502bbf16b8ae27af2bf8ac3b7cc10cfbd04ea",
    name: "gdx/v0.7.0-unstable-b710fae7/linux-amd64/go1.11.1",
    network: {
      inbound: false,
      localAddress: "192.168.0.109:50855",
      remoteAddress: "35.164.203.139:36000",
      static: true,
      trusted: false
    },
    protocols: {
      eth: {
        difficulty: 1048576,
        head: "0xfc27079037c07c16646518b7f5067ae5e07df0657c2e949ed84e0d6e2e4f7577",
        version: 64
      }
    }
}]
```

### 4.3.3. admin.nodeInfo

| Usage          | Description                     |
|----------------|---------------------------------|
| admin.nodeInfo | Detailed local node information |

Example:

```shell
> admin.nodeInfo
{
  enode: "enode://72c61072be163425f780ce1c54bf62835116bf471e8c6551dfb5bdb16128ab546c3bab8fc0f820e090d5cb523ca3ad2bc1d88435e6ce3b1e095c9893ff5ac4a9@192.168.7.99:36000",
  enr: "0xf89cb8403553c1046074ce413afdcd70e42429df7570c8c45855666b8f0a06af071503036e25b748834d8e7b3718dd50d02128a041b6fc1ad84b165019724d92e78a47220483636170ccc5836574683fc5836574684082696482763482697084c0a8076389736563703235366b31a10372c61072be163425f780ce1c54bf62835116bf471e8c6551dfb5bdb16128ab5483746370828ca083756470828ca0",
  id: "6ae04b801cf80310b48a10b55fbb478bfb977e159e96ea3b0a9d0a52ae8164ec",
  ip: "192.168.7.99",
  listenAddr: "[::]:36000",
  name: "gdx/v0.7.0-unstable-9437f7c7/darwin-amd64/go1.11.12",
  ports: {
    discovery: 36000,
    listener: 36000
  },
  protocols: {
    eth: {
      config: {
        byzantiumBlock: 0,
        chainId: 1,
        eip150Block: 0,
        eip150Hash: "0x41941023680923e0fe4d74a34bdac8141f2540e3ae90623718e47d66d1ca4a2d",
        eip155Block: 0,
        eip158Block: 0,
        ethash: {},
        homesteadBlock: 0
      },
      difficulty: 13055408,
      genesis: "0xfc27079037c07c16646518b7f5067ae5e07df0657c2e949ed84e0d6e2e4f7577",
      head: "0x14eaa4d7901af0b65fb815ee8623c0ffd11f762a39f73a6b887284480c2ce6c5",
      network: 1
    }
  }
}
```

## 4.4. StorageClient

**NOTE:** To be able to use the following commands, the storage client module must be enabled when you start the `gdx` program. Otherwise, error will be returned.

### 4.4.1. sclient.host.<span>ls

| Usage                 | Description                          |
|-----------------------|--------------------------------------|
| sclient.host.<span>ls | Retrieve the available storage hosts |

Example:

```
> sclient.host.ls
[{
    acceptingContracts: true,
    baseRPCPrice: 100000000000,
    contractPrice: 50000000000000000,
    deposit: 1000,
    downloadBandwidthPrice: 100000000,
    enodeid: "578c262796d0c566e968cef804e502bbf16b8ae27af2bf8ac3b7cc10cfbd04ea",
    enodeurl: "enode://fdac7145904eba296b91b21dbc2fb75c114ab3a4e2cf5076ea95ae278e142d9b40da8b7c061beb179523781b8d4207de69266862090ac7b24e3afa62e5bb9629@35.164.203.139:36000",
    filtered: false,
    firstseen: 15,
    historicdowntime: 0,
    historicfailedinteractions: 0,
    historicsuccessfulinteractions: 0,
    historicuptime: 0,
    ip: "35.164.203.139",
    ipnetwork: "35.164.203.0/24",
    lasthistoricupdate: 0,
    lastipnetworkchange: "2019-08-01T11:17:24.527496-07:00",
    maxDeposit: 9000000000000000000,
    maxDownloadBatchSize: 17825792,
    maxDuration: 172800,
    maxReviseBatchSize: 17825792,
    nodepubkey: "BP2scUWQTropa5GyHbwvt1wRSrOk4s9QduqVrieOFC2bQNqLfAYb6xeVI3gbjUIH3mkmaGIJCseyTjr6YuW7lik=",
    paymentAddress: "0x3c15440003892ad2a755459a66ccb3197234d7e2",
    recentfailedinteractions: 0,
    recentsuccessfulinteractions: 1,
    remainingStorage: 0,
    scanrecords: [{
        success: true,
        timestamp: "2019-08-01T11:07:24.576446-07:00"
    }, {
        success: true,
        timestamp: "2019-08-01T11:17:24.576447-07:00"
    }],
    sectorAccessPrice: 10000000000000,
    sectorSize: 4194304,
    storagePrice: 1000,
    totalStorage: 0,
    uploadBandwidthPrice: 10000000,
    version: "1.0.1",
    windowSize: 1200
}]
```

**NOTE:** length can be attach to the end of the command to get number of storage hosts that the client learnt from the network. If error returned, meaning that there are no hosts available.

```shell
> sclient.host.ls.length
1
```

### 4.4.2. sclient.paymentAddr

| Usage               | Description                                   |
|---------------------|-----------------------------------------------|
| sclient.paymentAddr | Retrieve the account used for storage service |

**NOTE:** By default, the first account user generated will be used as the storage service payment address. All the storage cost will be spent from this account.

Example:

```shell
> sclient.paymentAddr
"0x792e6b278ef8ec562b9530bf5df70064a55c3744"
```

### 4.4.3. sclient.setPaymentAddr

| Usage                             | Description                                                     |
|-----------------------------------|-----------------------------------------------------------------|
| sclient.setPaymentAddr(*address*) | Register the account address to be used for the storage service |

Example:

```shell
> sclient.setPaymentAddr(eth.accounts[1])
true
```

### 4.4.4. sclient.setConfig

| Usage                       | Description                                                       |
|-----------------------------|-------------------------------------------------------------------|
| sclient.setConfig(*config*) | Configure the client settings used for contract creation and etc. |

The following is a list of supported configuration:

| Config |    Type   | Description                                                                           |
|--------|-----------|---------------------------------------------------------------------------------------|
| period | Duration  | file storage duration                                                                 |
| hosts  | Number    | number of storage hosts that the client want to sign contract with                    |
| renew  | Duration  | time that the contract will automatically be renewed                                  |
| fund   | Currency  | amount of money the client wants to be used for the storage service within one period |


**NOTE:** units must be included when config the storage client configuration, please refer to [Section 6.1 Units](#section-61-units) for available units

Before setting the client config, please make sure that you have enough balance in your payment account. Please refer to [4.1.4. eth.getBalance](#414-ethgetbalance) to check balance. 

In addition, the payment account must be permanently unlocked first. To unlock the account, please refer to [4.1.3. personal.unlockAccount](#413-personalunlockaccount)


Example:

```shell
> sclient.setConfig({"fund":"1dx", "hosts":"3", "period":"2d", "renew":"36h"})
"Successfully set the storage client setting"
```

the following command uses the default settings

```shell
> sclient.setConfig({})
"Successfully set the storage client setting"
```

Default Setting:

```
Fund:   1 DX
Hosts:  3 Hosts
Renew:  36 Hours
Period: 2 Days
```

**NOTE:** once the client configuration is settled, the contracts will be automatically created.

### 4.4.5. sclient.config

| Usage          | Description                                                  |
|----------------|--------------------------------------------------------------|
| sclient.config | Current storage client settings used for the storage service |

Example:

```shell
> sclient.config
{
  IP Violation Check Status: "Disabled: storage client can sign contract with storage hosts from the same network",
  Max Download Speed: "Unlimited",
  Max Upload Speed: "Unlimited",
  RentPayment Setting: {
    Expected Download: "578703 B/block",
    Expected Redundancy: "2 Copies",
    Expected Storage: "976562500 KiB",
    Expected Upload: "1157407 B/block",
    Fund: "1 DX",
    Number of Storage Hosts: "3 Hosts",
    Renew Time: "12 Hour(s)",
    Storage Time: "3 Day(s)"
  }
}
```

### 4.4.6. sclient.contracts

| Usage             | Description                                           |
|-------------------|-------------------------------------------------------|
| sclient.contracts | Active storage contracts signed by the storage client |

Example:

```shell
> sclient.contracts
[{
    AbleToRenew: true,
    AbleToUpload: true,
    Canceled: false,
    ContractID: "0x527924bc4e7316382601662527dbc327a7854918bed725522a633bd383257df0",
    HostID: "578c262796d0c566e968cef804e502bbf16b8ae27af2bf8ac3b7cc10cfbd04ea"
}]
```

to get the number of contracts that the storage client signed

```shell
> sclient.contracts.length
1
```

### 4.4.7 sclient.contract

| Usage                        | Description                                                     |
|------------------------------|-----------------------------------------------------------------|
| sclient.contract(*contractID*) | Detailed contract information based on the provided contract ID |

Example:

```shell
> sclient.contract("0x527924bc4e7316382601662527dbc327a7854918bed725522a633bd383257df0")
{
  Canceled: "the contract is still active",
  ContractBalance: "0.06111111111111111 DX",
  ContractFee: "0.05 DX",
  DownloadCost: "0 Camel",
  EndHeight: "17843 b",
  EnodeID: "578c262796d0c566e968cef804e502bbf16b8ae27af2bf8ac3b7cc10cfbd04ea",
  GasCost: "0 Camel",
  ID: "0x527924bc4e7316382601662527dbc327a7854918bed725522a633bd383257df0",
  LatestContractRevision: {
    Signatures: ["BUl14sSzxbciCt4Wg/9ouy80qygRRQIKbDwGOfG/MxtnDh5m2b+ENR+hqCglInyTDdsO6/vqwoaimFves2jUMgA=", "Seh6lPZTnp8yI0bjc3x44gIgnu+dcgj4aonMV6gF0Ap21qCMMhqino2ueghtVbZyILANAkhdbwW36TgggL0g1gA="],
    newfilemerkleroot: "0x0000000000000000000000000000000000000000000000000000000000000000",
    newfilesize: 0,
    newmissedproofoutputs: [{
        Address: "0x792e6b278ef8ec562b9530bf5df70064a55c3744",
        Value: 61111111111111110
    }, {
        Address: "0x3c15440003892ad2a755459a66ccb3197234d7e2",
        Value: 111111111111111000
    }],
    newrevisionnumber: 1,
    newunlockhash: "0x2f58aa5ab70e1d6844d829948070b34b5f4bcf93cdc66dac72bf8e3b6ae56f26",
    newvalidproofoutputs: [{
        Address: "0x792e6b278ef8ec562b9530bf5df70064a55c3744",
        Value: 61111111111111110
    }, {
        Address: "0x3c15440003892ad2a755459a66ccb3197234d7e2",
        Value: 111111111111111000
    }],
    newwindowend: 19043,
    newwindowstart: 17843,
    parentid: "0x527924bc4e7316382601662527dbc327a7854918bed725522a633bd383257df0",
    unlockconditions: {
      paymentaddress: ["0x792e6b278ef8ec562b9530bf5df70064a55c3744", "0x3c15440003892ad2a755459a66ccb3197234d7e2"],
      signaturesrequired: 2
    }
  },
  RenewAbility: "the contract can be used for data downloading",
  StartHeight: "563 b",
  StorageCost: "0 Camel",
  TotalCost: "0.1111111111111111 DX",
  UploadAbility: "the contract can be used for data uploading",
  UploadCost: "0 Camel"
}
```

### 4.4.8 sclient.upload

| Usage                                   | Description                                                      |
|-----------------------------------------|------------------------------------------------------------------|
| sclient.upload(*source*, *destination*) | Upload the file specified by the storage client to storage hosts |

**NOTE:** both source and destination path must be absolute path

Example:

```shell
> sclient.upload("/Users/mzhang/upload1.file", "download.file")
"success"
```

### 4.4.9 sclient.download

| Usage                                     | Description                                                    |
|-------------------------------------------|----------------------------------------------------------------|
| sclient.download(*source*, *destination*) | Download the file specified by the client to the local machine |

**NOTE:** both source and destination path must be absolute path

Example:

```shell
> sclient.download("download.file", "download.file")
"File downloaded successfully"
```

### 4.4.10 sclient.file.<span>ls

| Usage                 | Description                          |
|-----------------------|--------------------------------------|
| sclient.file.<span>ls | Files uploaded by the storage client |


Example:

```shell
> sclient.file.ls
[{
    dxpath: "download.file",
    status: "healthy",
    uploadProgress: 100
}]
```

### 4.4.11 sclient.file.rename

| Usage                                     | Description              |
|-------------------------------------------|--------------------------|
| sclient.file.rename(*oldName*, *newName*) | Rename the file uploaded |

Example:

```shell
> sclient.file.rename("download.file", "newfile.file")
"File download.file renamed to newfile.file"
```

### 4.4.12 sclient.file.delete

| Usage                       | Description     |
|-----------------------------|-----------------|
| sclient.file.delete(*file*) | Delete the file |

Example:

```shell
> sclient.file.delete("newfile.file")
"File newfile.file deleted"
```


## 4.5. StorageHost

**NOTE:** To be able to use the following commands, the storage host module must be enabled when you start the `gdx` program. Otherwise, error will be returned.

### 4.5.1. shost.config

| Usage        | Description                                          |
|--------------|------------------------------------------------------|
| shost.config | Storage host configurations used for storage service |

**NOTE:** if the configuration is not set, default configuration will be used.

Example:

```shell
> shost.config
{
  acceptingContracts: "false",
  baseRPCPrice: "100 Gcamel",
  contractPrice: "0.05 DX/contract",
  deposit: "1000 Camel/byte/block",
  depositBudget: "10000 DX/contract",
  downloadBandwidthPrice: "0.1 Gcamel/byte",
  maxDeposit: "100 DX",
  maxDownloadBatchSize: "17 MiB/block",
  maxDuration: "1 Month(s)",
  maxReviseBatchSize: "17 MiB/block",
  paymentAddress: "0x0000000000000000000000000000000000000000",
  sectorAccessPrice: "10000 Gcamel/sector",
  storagePrice: "1000 Camel/byte/block",
  uploadBandwidthPrice: "0.01 Gcamel/byte",
  windowSize: "5 Hour(s)"
}
```

### 4.5.2. shost.setConfig

| Usage                     | Description                         |
|---------------------------|-------------------------------------|
| shost.setConfig(*config*) | Set the storage host configurations |

The following is a list of available configurations:

| Config             | Type     | Description                                           |
|--------------------|----------|-------------------------------------------------------|
| acceptingContracts | Boolean  | whether the host accepts new contracts                |
| maxDuration        | Duration | max duration for a storage contract                   |
| deposit            | Currency | deposit price per block per byte                      |
| contractPrice      | Currency | price that client must be paid when creating contract |
| downloadPrice      | Currency | download bandwidth price per byte                     |
| uploadPrice        | Currency | upload bandwidth price per byte                       |
| storagePrice       | Currency | storage price per block per byte                      |
| depositBudget      | Currency | the maximum deposit for all contracts                 |
| maxDeposit         | Currency | the max deposit for a single storage contract         |
| paymentAddress     | Address  | account address used for the storage service          |

**NOTE** for available units, please refer to [Section 6.1 Units](#section-61-units)

Example:

```shell
> shost.setConfig({"acceptingContracts":"true", "deposit":"2gcamel"})
"Successfully set the host config"
```

### 4.5.3. shost.paymentAddr

| Usage             | Description                                   |
|-------------------|-----------------------------------------------|
| shost.paymentAddr | Retrieve the account used for storage service |

**NOTE:** By default, the first account user generated will be used as the storage service payment address. All the storage cost will be spent from this account. All the profit will be saved in this account as well

Example:

```shell
> shost.paymentAddr
"0x792e6b278ef8ec562b9530bf5df70064a55c3744"
```

### 4.5.4. shost.announce

| Usage          | Description                            |
|----------------|----------------------------------------|
| shost.announce | Announce the node as storage host node |

**NOTE:** there is no limitation on number of announcements. However, each announcement will cost a small amount of transaction fee. Before making announcement, please make sure that you have enough balance in your payment account. Please refer to [4.1.4. eth.getBalance](#414-ethgetbalance) to check balance. 

In addition, the payment account must be permanently unlocked first. To unlock the account, please refer to [4.1.3. personal.unlockAccount](#413-personalunlockaccount)

Example:

```shell
> shost.announce()
"Announcement transaction: 0x3e3466bab9476d997ab4fc8a8a68c0893ecd373646e6bbc98137cd343b169881"
```

to check the transaction detail, use the following command

```shell
> eth.getTransaction("0x3e3466bab9476d997ab4fc8a8a68c0893ecd373646e6bbc98137cd343b169881")
{
  blockHash: "0xcd3f13a85c129ebc4d77d4b3c65db7a37921e3c16d40e816daf0cacb32e6191f",
  blockNumber: 1745,
  from: "0x792e6b278ef8ec562b9530bf5df70064a55c3744",
  gas: 90000,
  gasPrice: 1000000000,
  hash: "0x3e3466bab9476d997ab4fc8a8a68c0893ecd373646e6bbc98137cd343b169881",
  input: "0xf8e0b89b656e6f64653a2f2f3936333438326236643764353433623335316439396464633833373031336166336239646438363230346465623930313165306663363561656163383039666137353833663061633063323261366262633364383861343262346430346266373262326265373635653039653164643763313664396237646432623563336166403139322e3136382e372e39393a3336303030b8415978fcb1e3d366450f1dfde82a8aab4e1dc5c8675f79314dc0a6f13ebfe007bd73e5e16c92323f32d073e09404889ad8939c3b0e35b3bfa536b80a3d93dc645a00",
  nonce: 2,
  r: "0xc579aa9e6bbcbf7e466c39d8d077a5b9d5c1de73f9a4a598689c88eb6869ae74",
  s: "0xf980116eb2a47fc7f18054fd87bcdbef8dc8f67b7eb2b17a283d744e2704951",
  to: "0x0000000000000000000000000000000000000009",
  transactionIndex: 0,
  v: "0x25",
  value: 0
}
```

### 4.5.5. shost.folder.add

| Usage                            | Description                                                        |
|----------------------------------|--------------------------------------------------------------------|
| shost.folder.add(*path*, *size*) | Allocate disk space for saving data uploaded by the storage client |

**NOTE:** for supported storage size unit, please refer to [Section 6.1 Units](#section-61-units)

Example:

```shell
> shost.folder.add("~/temp", "1gb")
"successfully added the storage folder"
```

### 4.5.6. shost.folder.<span>ls

| Usage                 | Description                                                        |
|-----------------------|--------------------------------------------------------------------|
| shost.folder.<span>ls | Information of folders created for storing data uploaded by client |

Example:

```shell
> shost.folder.ls
[{
    path: "/Users/mzhang/temp",
    totalSectors: 238,
    usedSectors: 0
}]
```

### 4.5.7. shost.folder.resize

| Usage                                    | Description                                                                    |
|------------------------------------------|--------------------------------------------------------------------------------|
| shost.folder.resize(*directory*, *size*) | Resize the disk space allocated for saving data uploaded by the storage client |

Example:

```shell
> shost.folder.resize("/Users/mzhang/temp", "500mb")
"successfully resize the storage folder"
```

### 4.5.8. shost.folder.delete

| Usage                            | Description                                                                |
|----------------------------------|----------------------------------------------------------------------------|
| shost.folder.delete(*directory*) | Free up the disk space used for saving data uploaded by the storage client |

**NOTE:** this command will not delete the directory created on your local machine

Example:

```shell
> shost.folder.delete("/Users/mzhang/temp")
"successfully delete the storage folder"
```

# Section 5. License

GoDx is released under the [Apache 2.0 License](https://opensource.org/licenses/Apache-2.0). See LICENSE for more information.

# Section 6. Appendix

## Section 6.1 Units

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

# Section 7. Contact

Thank you so much for your support and your confidence in this project. If you have any question, please do not hesitated to contact us via support@dxchain.com