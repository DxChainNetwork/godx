# Windows Version Setup

## 1. Prerequisites

### 1.1. Golang Installation

To build the program from the source code, Golang 1.11 is required. Please follow [this link](https://golang.org/doc/install) to install Golang on your computer (make sure to choose the windows distribution). To check if the program is installed successfully, use the following command:

```shell
$ go version
```

### 1.2. Git Bash Installation

Git-bash will be the terminal used to execute the commands mentioned in the later section. Please follow [this link](https://gitforwindows.org) to download the git bash for windows. 

### 1.3. TDM-GCC Installation

Please follow [this link](https://sourceforge.net/projects/tdm-gcc/) to download the TDM-GCC MinGW Compiler.

### 1.4. `make` Command Installation

Please follow [this link](https://sourceforge.net/projects/gnuwin32/files/make/3.81/make-3.81.exe/download?use_mirror=iweb&download=) to install `make` command. After the installation is done, add the path `C:\Program Files (x86)/GnuWin32/bin` into the system environment path. (if you do not know how to set it, you can check [this article](https://www.java.com/en/download/help/path.xml))

### 1.5. Docker (Optional)

The installation of Docker is optional. Docker is used for cross-platform build meaning if you want to build linux version of `gdx` on the windows operating system, you have to install Docker. Please follow the following installation guide:

[Docker Installation Guide for Windows](https://docs.docker.com/docker-for-windows/install/)

## 2. Build from source

### 2.1. Get the source code

Open the git bash you downloaded for the prerequisites and execute the command below

```shell
$ go get -u github.com/DxChainNetwork/godx
```

### 2.2. Packages Installation

Please run the following command in git bash to install the dependencies for the `gdx` executable

```shell
$ go get -u -v github.com/kardianos/govendor
$ cd ~/go/src/github.com/DxChainNetwork/godx
$ govendor sync
```

### 2.3. Build

Please run the following command in git bash to build the executable.

```shell
$ make gdx
```
