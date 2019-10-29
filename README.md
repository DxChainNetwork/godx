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
- [Section 2. Running `gdx`](#section-2-running-gdx)
- [Section 3. Tutorial](#section-3-tutorial)
- [License](#license)
- [Appendix](#appendix)
  - [Templates](#templates)
- [Contact](#contact)

# Section 1. Getting Started

-  For MacOS or Linux, please click [here](./LINUX.md)
-  For Windows, please click [here](./WINDOWS.md)

# Section 2. Running `gdx`

To run the node, please use the following command:

```shell
$ gdx
```

> for windows, please replace gdx with /c/Users/${USERNAME}/go/src/github.com/DxChainNetwork/godx/build/bin/gdx
>  NOTE: please replace  ${USERNAME} with your actual username

To open the gdx console, please run the following command:

```shell
$ gdx attach
```

> for windows, please replace gdx with /c/Users/${USERNAME}/go/src/github.com/DxChainNetwork/godx/build/bin/gdx
>  NOTE: please replace  ${USERNAME} with your actual username

# Section 3. Tutorial

> Before looking through specific tutorials, please following the [preparation instructions](https://github.com/DxChainNetwork/godx-doc/blob/master/gdx/gdx-manual/manual_en.md) first

For the storage tutorial, please click [here](https://github.com/DxChainNetwork/godx-doc/blob/master/gdx/gdx-manual/storage_manual/storage_en.md)

For the DPoS tutorial, please click [here](https://github.com/DxChainNetwork/godx-doc/blob/master/gdx/gdx-manual/dpos_manual/dpos_en.md)

# License

GoDx is released under the [Apache 2.0 License](https://opensource.org/licenses/Apache-2.0). See LICENSE for more information.

# Appendix

## Templates

* To form a bug report, [Bug Report Template](./.github/ISSUE_TEMPLATE/bug_report.md) must be followed
* To request a new feature, [Feature Request Template](./.github/ISSUE_TEMPLATE/feature_request.md) must be followed
* To submit a pull request, [Pull Request Template](./.github/PULL_REQUEST_TEMPLATE/pull_request_template.md) must be followed

Contribution is welcome, see [Contributing](./CONTRIBUTING.md) for more details

# Contact

Thank you so much for your support and your confidence in this project. If you have any question, please do not hesitated to contact us via support@dxchain.com