# Go DxChain Tutorial


**Table of Contents:**

- [Go DxChain Tutorial](#go-dxchain-tutorial)
- [Section One: Miner](#section-one-miner)
    - [Step1.1. Start Node](#step11-start-node)
    - [Step1.2. Open `gdx` Console](#step12-open-gdx-console)
    - [Step1.3. Create Account and Start Mining](#step13-create-account-and-start-mining)
    - [Step1.4. Stop Mining](#step14-stop-mining)
- [Section Two: Storage Client](#section-two-storage-client)
  - [Storage Client Execution Steps](#storage-client-execution-steps)
    - [Step2.1. Node Termination](#step21-node-termination)
    - [Step2.2. Start Node](#step22-start-node)
    - [Step2.3. Open `gdx` Console](#step23-open-gdx-console)
    - [Step2.4. Create Account and Start Mining](#step24-create-account-and-start-mining)
    - [Step2.5. Unlock Account](#step25-unlock-account)
    - [Step2.6. Check Balance](#step26-check-balance)
    - [Step2.7. Client Configuration](#step27-client-configuration)
    - [Step2.8. File Upload](#step28-file-upload)
    - [Step2.9. Upload Progress](#step29-upload-progress)
    - [Step2.10. File Download](#step210-file-download)
    - [Step2.11. File Verification](#step211-file-verification)
- [Section Three: Storage Host](#section-three-storage-host)
  - [Storage Host Execution Steps](#storage-host-execution-steps)
    - [Step3.1. Node Termination](#step31-node-termination)
    - [Step3.2. Start Node](#step32-start-node)
    - [Step3.3. Open `gdx` Console](#step33-open-gdx-console)
    - [Step3.4. Create Account and Start Mining](#step34-create-account-and-start-mining)
    - [Step3.5. Unlock Account](#step35-unlock-account)
    - [Step3.6. Check Balance](#step36-check-balance)
    - [Step3.7. Add Folder](#step37-add-folder)
    - [Step3.8. Host Announcement](#step38-host-announcement)
- [Section Four: Program Clean Start](#section-four-program-clean-start)


# Section One: Miner

Every node can become a miner regardless of its role. If you do not intend to become neither a storage client nor a storage host, you can start the program by running the following command in the terminal.

```shell
$ gdx --role miner
```

**NOTE: Both storage client and storage host have full functionality of miner.**

### Step1.1. Start Node

To start the node as a miner, please type the following commands in the terminal

```shell
$ gdx --role miner
```

### Step1.2. Open `gdx` Console

Open another terminal panel and start the gdx console by using the following command

```shell
$ gdx attach
```

### Step1.3. Create Account and Start Mining

To acquire tokens, you will first need to create an account and start mining. Typing the following commands in the `gdx` console you just opened.

```js
> personal.newAccount("")
> miner.start()
```

**NOTE: for simplification, the password used in this command is empty.**


### Step1.4. Stop Mining

To stop mining, simply type the following commands in the `gdx` console.

```js
> miner.stop()
```

# Section Two: Storage Client

By paying DX tokens to storage hosts, storage client is able to rent storage space and store files in the DX network safely and securely. When needed, storage client can download those files from the network.

To start the node as a storage client only, run the following command in the terminal

```shell
$ gdx --role storageclient
```

## Storage Client Execution Steps

### Step2.1. Node Termination

**Note: skip step 2.1 if you have not yet to run the `gdx` program**

If the node is running or had ran previously, you must terminate the program by pressing `Ctrl + C`, and then [clean start](#Section-Four-Program-Clean-Start) the node

### Step2.2. Start Node

To start the node as the storage client, type the following commands in the terminal

```shell
$ gdx --role client
```

### Step2.3. Open `gdx` Console

Open another terminal panel and start the gdx console by using the following command

```shell
$ gdx attach
```

### Step2.4. Create Account and Start Mining

In order to create contract and start to upload file, you must have some tokens first. To get tokens, you will first need to create an account and start mining. Typing the following commands in the `gdx` console you just opened.

```js
> personal.newAccount("")
> miner.start()
```

**NOTE: for simplification, the password used in this command is empty**

### Step2.5. Unlock Account

If the account is locked, you will not be allowed to make any transaction. Therefore, we need to unlock the account by typing the following command in the console.

```js
> personal.unlockAccount(eth.accounts[0], "", 0)
```

### Step2.6. Check Balance

As mentioned above, to create contract and then to upload files, you must have enough tokens. You can use the following command in the console to check the account balance. Wait until the result is not 0, and then you can proceed to the next step

```js
> eth.getBalance(eth.accounts[0])
```

**NOTE: Before you proceed to _Step3.7. Client Configuration_, please make sure that you have connected to at least 3 hosts. You can run the following code to get hosts number**
```shell
sclient.host.ls
```

### Step2.7. Client Configuration

To create contracts with storage hosts, you must specify the client configurations. By typing the following command in the console, the default configuration will be used

```js
> sclient.setConfig({})
```

Wait until 3 contracts are created. You can check number of contracts created by using the following command:

```js
> sclient.contracts.length
```

If error shows up, it means you haven't create any contract yet. It may take a while since you need to sync.

### Step2.8. File Upload

By typing the following commands in the terminal, a file named `upload.file` with random data will be created under the home directory. The file size is 512KB.

```shell
$ cd ~
$ dd if=/dev/urandom of=upload.file count=512 bs=1024
```

Then, get the absolute path of the file by using the following command  

```shell
$ cd ~
$ ls "`pwd`/upload.file"
```

In the `gdx` console, use the following command to upload the file.  
**Note: replace `FILE_PATH` with the absolute path of the file**

```js
> sclient.upload("FILE_PATH", "download.file")
```

### Step2.9. Upload Progress

To check the upload progress, type the following command in the console:

```js
> sclient.file.ls
```

Wait util the upload progress reaches 100%, then you can proceed to the next step.

### Step2.10. File Download

When the file is uploaded, you will be able to download it. To download the file to the home directory, you need to first get the absolute path of the home directory by typing the following command in the terminal

```shh
$ cd ~
$ pwd
```

Then, typing the following command in the `gdx` console (**NOTE: replace `FILE_PATH` with the absolute path of the home directory**)

```js
> sclient.download("download.file", "FILE_PATH/download.file")
```

The file will be downloaded to your home directory as `download.file`

### Step2.11. File Verification

Since the file uploaded is generated and filled in with random data. To check if the file downloaded is the file you actually uploaded, use the following commands

```shell
$ cd ~
$ shasum -a 256 upload.file
$ shasum -a 256 download.file
```

If the hash value you got for upload.file matches with the hash value you got from download.file. It means two files are exactly the same.

# Section Three: Storage Host

Storage host serves as storage service provider, gaining profit for storing data uploaded by the storage client.

To start the node as a storage host, run the following command in the terminal

```shell
$ gdx --role host
```

**NOTE: a node can either become a storage client or a storage host. To switch between two roles, you must [clean start](#Section-Four-Program-Clean-Start) the node**
## Storage Host Execution Steps

### Step3.1. Node Termination

**Note: skip step 3.1 if you have not yet to run the `gdx` program**

If the node is running or had ran previously, you must terminate the program by pressing `Ctrl + C`, and then [clean start](#Section-Four-Program-Clean-Start) the node

### Step3.2. Start Node

To start the node as the storage host, type the following commands in the terminal

```shell
$ gdx --role host
```

### Step3.3. Open `gdx` Console

Open another terminal panel and start the gdx console by using the following command

```shell
$ gdx attach
```

### Step3.4. Create Account and Start Mining

In order to create contract with clients, you must have some tokens first. To get tokens, you will first need to create an account and start mining. Typing the following commands in the `gdx` console you just opened.

```js
> personal.newAccount("")
> miner.start()
```

**NOTE: for simplification, the password used in this command is empty.**

### Step3.5. Unlock Account

If the account is locked, you will not be allowed to make any transaction. Therefore, we need to unlock the account by typing the following command in the console

```js
> personal.unlockAccount(eth.accounts[0], "", 0)
```

### Step3.6. Check Balance

As mentioned above, to create contract, you must have some tokens. Using the following command in the console to check the balance. Wait until the result is not 0, and then you can proceed to the next step

```js
> eth.getBalance(eth.accounts[0])
```

### Step3.7. Add Folder

To be able to save the data uploaded by the storage client, you must allocate disk space first. In the `gdx` console, type the following command to allocate `1 GB` disk space under the home directory, `temp` folder (Note: the folder `temp/dxchain` will be created under the home directory along with some files)

```js
> shost.folder.add("~/temp/dxchain", "1gb")
```

### Step3.8. Host Announcement

Lastly, announcement must be made to let other nodes knew that your node is a storage host node. By using the console command below, storage client will identify you as a storage host. (NOTE: you need to pay gas fee for doing this operation)

```js
> shost.announce()
```

# Section Four: Program Clean Start

To start the node as a new node, the previous saved data must be removed (NOTE: this means everything will be started from the beginning)

- For macOS: `rm -r ~/Library/DxChain`
- For linux: `rm -r ~/.dxchain`

Once the old data got removed, the node can be started as either storage client or storage host

