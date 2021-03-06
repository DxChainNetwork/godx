package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/DxChainNetwork/godx/cmd/utils"
	"github.com/DxChainNetwork/godx/common"
	"github.com/DxChainNetwork/godx/node"
	"github.com/DxChainNetwork/godx/rpc"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"github.com/olekukonko/tablewriter"

	"gopkg.in/urfave/cli.v1"
)

var (
	storageHostIDFlag = cli.StringFlag{
		Name:  "hostid",
		Usage: "Storage host enode ID",
	}

	contractIDFlag = cli.StringFlag{
		Name:  "contractid",
		Usage: "Contract ID ",
	}

	paymentAddressFlag = cli.StringFlag{
		Name:  "address",
		Usage: "Payment address for the storage service",
	}

	contractPeriodFlag = cli.StringFlag{
		Name:  "period",
		Usage: "Duration of data storage ",
	}

	contractHostFlag = cli.StringFlag{
		Name:  "host",
		Usage: "Number of hosts that storage client wants to sign the contract with",
	}

	contractRenewFlag = cli.StringFlag{
		Name:  "renew",
		Usage: "Time for automatic contract renew",
	}

	contractFundFlag = cli.StringFlag{
		Name:  "fund",
		Usage: "Money can be spent for the file storage within in one period",
	}

	fileSourceFlag = cli.StringFlag{
		Name:  "src",
		Usage: "Absolute path of the file that is going to be uploaded/downloaded from (source)",
	}

	fileDestinationFlag = cli.StringFlag{
		Name:  "dst",
		Usage: "Absolute path of the file that is going to ge uploaded/downloaded to (destination)",
	}

	filePathFlag = cli.StringFlag{
		Name:  "filepath",
		Usage: "Absolute path of the file",
	}

	prevFilePathFlag = cli.StringFlag{
		Name:  "prevpath",
		Usage: "Previous absolute file path",
	}

	newFilePathFlag = cli.StringFlag{
		Name:  "newpath",
		Usage: "New absolute file path",
	}
)

var storageClientCommand = cli.Command{
	Name:      "sclient",
	Usage:     "Storage client related operations",
	ArgsUsage: "",
	Category:  "STORAGE CLIENT COMMANDS",
	Description: `
   		gdx storage client commands
	`,

	Subcommands: []cli.Command{
		{
			Name:      "config",
			Usage:     "Retrieve storage client configurations",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getConfig),
			Description: `
			gdx sclient config

will display the current storage client settings used for the storage service,
including but not limited to storage time, automatically contract renew time, etc.`,
		},

		{
			Name:      "hosts",
			Usage:     "Retrieve a list of storage hosts",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getHosts),
			Description: `
			gdx sclient hosts

will display a list of storage hosts that the client can sign contract with. The program
will automatically evaluate storage hosts from this list to sign contract with them`,
		},

		{
			Name:      "host",
			Usage:     "Retrieve detailed host information based on the provided hostID",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getHostInfo),
			Flags: []cli.Flag{
				storageHostIDFlag,
			},
			Description: `
			gdx sclient host [--hostid arg]

will display detailed host information based on the provided hostID, such as deposit,
allowed storage time, and etc.`,
		},

		{
			Name:      "hostrank",
			Usage:     "Retrieve host's ranking status for each storage host learnt by the client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getRanking),
			Description: `
			gdx sclient hostrank

will display display detailed host's ranking status including detailed evaluation for 
each of the storage host`,
		},

		{
			Name:      "contracts",
			Usage:     "Retrieve all active storage contracts signed by the client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getContracts),
			Description: `
			gdx sclient contracts

will display all active storage contracts signed by the client along with the basic information
of each signed storage contract, such as contract status, contractID, and hostID that client
signed the contract with`,
		},

		{
			Name:      "files",
			Usage:     "Retrieve all files uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getFiles),
			Description: `
			gdx sclient files

will display all files uploaded by the storage client along with the basic information for
each file, including the file's uploading status and health status'`,
		},

		{
			Name:      "contract",
			Usage:     "Retrieve detailed contract information of a contract ",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getContract),
			Flags: []cli.Flag{
				contractIDFlag,
			},
			Description: `
			gdx sclient contract [--contractid arg]

will display detailed contract information based on the provided contractID. The information
included contractID, revisionNumber, hostID, and etc.'`,
		},

		{
			Name:      "paymentAddr",
			Usage:     "Retrieve the account address used for storage service payment",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getPaymentAddress),
			Description: `
			gdx sclient paymentAddr
		
will display the the account address used for the storage service. Unless user set it specifically,
the payment address for the storage service will always be the first account address`,
		},
		{
			Name:      "setPaymentAddr",
			Usage:     "Register the account address to be used for the storage services",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setPaymentAddress),
			Flags: []cli.Flag{
				paymentAddressFlag,
			},
			Description: `
			gdx sclient setPaymentAddr [--address arg]
		
is used to register the account address to be used for the storage services. Money spent for the
file uploading, downloading, storage, and etc. will be deducted from this address. The --address
flag must be used along with this flag to specify the account address`,
		},

		{
			Name:      "setConfig",
			Usage:     "Configure the client settings used for contract creation, file upload, download, and etc.",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setClientConfig),
			Flags: []cli.Flag{
				contractPeriodFlag,
				contractHostFlag,
				contractFundFlag,
			},
			Description: `
			gdx sclient setConfig [--period arg] [--host arg] [--fund arg]
		
will configure the client settings used for contract creation, file upload, download, and etc. There are
multiple flags can be used along with this command to specify the setting:
1. period: specifies the file storage time
2. host: specifies the number of storage hosts that the client want to sign contracts with
3. fund: specifies the amount of money the client wants to be used for the storage service

units:
currency: [camel, gcamel, dx]
time: [h, b, d, w, m, y] -> hour, block, day, week, month, year

Note: without using any of those flags, default settings will be used`,
		},

		{
			Name:      "upload",
			Usage:     "Upload the file from the local machine",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(fileUpload),
			Flags: []cli.Flag{
				fileSourceFlag,
				fileDestinationFlag,
			},
			Description: `
			gdx sclient upload [--src arg] [--dst arg]
		
will upload the file specified by the client to the storage hosts. This command must be used along
with two flags to specify the source of the file that is going to be uploaded, and the destination
that the file is going to be uploaded to. Note: the src must be absolute path: /home/ubuntu/upload.file`,
		},

		{
			Name:      "download",
			Usage:     "Download file to the local machine",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(fileDownload),
			Flags: []cli.Flag{
				fileSourceFlag,
				fileDestinationFlag,
			},
			Description: `
			gdx sclient download [--src arg] [--dst arg]

will download the file specified by the client to the local machine. This command must be used along
with two flags to specify the source of the file that is going to be downloaded, and the destination
that the file is going to be downloaded from. Note, the download destination must be absolute path.`,
		},

		{
			Name:      "file",
			Usage:     "Retrieve detailed information of an uploaded/uploading file",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getFile),
			Flags: []cli.Flag{
				filePathFlag,
			},
			Description: `
			gdx sclient file [--filepath arg]

will display the detailed information of an uploaded/uploading file, including the file uploading
status, health status, and etc. Note, the filepath must be specified which is the destination path
used for file uploading`,
		},

		{
			Name:      "rename",
			Usage:     "Rename the file uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(fileRenaming),
			Flags: []cli.Flag{
				prevFilePathFlag,
				newFilePathFlag,
			},
			Description: `
			gdx sclient rename [--prevpath arg] [--newpath arg]

will rename the file uploaded by the client. oldname and newname flags must be used along
with this command`,
		},

		{
			Name:      "delete",
			Usage:     "Rename the file uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(fileDelete),
			Flags: []cli.Flag{
				filePathFlag,
			},
			Description: `
			gdx sclient delete [--filepath arg]

will delete the file uploaded by the storage client. This filepath flag must be used along
with this command to specify which file will be deleted`,
		},
		{
			Name:      "periodCost",
			Usage:     "Retrieve the client's period cost for all storage contracts",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(periodCost),
			Description: `
			gdx sclient periodCost

will retrieve and display the cost that storage client needs to pay within one period cycle. 
The cost includes cost for all contracts. In addition, it also provides the contract fund left,
fund unspent, and fund withhold, along with the withhold fund release block height`,
		},
	},
}

func getConfig(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var config storage.ClientSettingAPIDisplay
	err = client.Call(&config, "sclient_config")
	if err != nil {
		utils.Fatalf("failed to get the storage client configuration: %s", err.Error())
	}

	fmt.Printf(`Client Configuration:
	Fund:                           %s
	Period:                         %s
	HostsNeeded:                    %s
	Redundancy:                     %s
	ExpectedStorage:                %s
	ExpectedUpload:                 %s
	ExpecedDownload:                %s
	Max Upload Speed:               %s
	Max Download Speed:             %s
	IP Violation Check Status:      %s
`, config.RentPayment.Fund, config.RentPayment.Period, config.RentPayment.StorageHosts,
		config.RentPayment.ExpectedRedundancy, config.RentPayment.ExpectedStorage, config.RentPayment.ExpectedUpload,
		config.RentPayment.ExpectedDownload, config.MaxUploadSpeed, config.MaxDownloadSpeed, config.EnableIPViolation)

	return nil
}

func getHosts(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var allStorageHosts []storage.HostInfo

	// get all storage hosts
	err = client.Call(&allStorageHosts, "sclient_hosts")
	if err != nil {
		utils.Fatalf("unable to get all storage host information: %s", err.Error())
	}

	if len(allStorageHosts) == 0 {
		fmt.Println("No storage hosts can be found")
		return nil
	}

	fmt.Println("Number of storage hosts: ", len(allStorageHosts))

	table := hostInfoTable(allStorageHosts)
	table.Render()
	fmt.Println()
	return nil
}

func getHostInfo(ctx *cli.Context) error {
	var id string
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	if !ctx.IsSet(storageHostIDFlag.Name) {
		utils.Fatalf("the --hostid flag must be used to specify which storage host information want to be retrieved")
	} else {
		id = ctx.String(storageHostIDFlag.Name)
	}

	var info storage.HostInfo

	// Specific Host Information
	err = client.Call(&info, "sclient_host", id)
	if err != nil {
		utils.Fatalf("failed to retrieve the storage host information: %s", err.Error())
	}

	if info.IP == "" {
		fmt.Println("the enode ID you entered does not exist")
		return nil
	}

	fmt.Printf(`Host Information:
	HostID:                        %s
	IP:                            %s
	AcceptingStorageContracts:     %t
	RemainingStorage:              %v bytes
	Deposit:                       %v camel
	Contract Price:                %v camel
	Storage Price:                 %v camel
	DownloadBandwidth Price:       %v camel
	UploadBandwidth Price:         %v camel
	Sector Access Price:           %v camel
	
`, info.EnodeID.String(), info.IP, info.AcceptingContracts, info.RemainingStorage, info.Deposit, info.ContractPrice,
		info.StoragePrice, info.DownloadBandwidthPrice, info.UploadBandwidthPrice, info.SectorAccessPrice)

	return nil
}

func getRanking(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var rankings []storagehostmanager.StorageHostRank
	err = client.Call(&rankings, "sclient_hostRank")
	if err != nil {
		utils.Fatalf("failed to retrieve the storage host rankings: %s", err.Error())
	}

	if len(rankings) == 0 {
		fmt.Println("No storage host can be found")
		return nil
	}

	table := hostRankingTable(rankings)
	table.Render()
	fmt.Println()
	return nil
}

func getContracts(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var contracts []storageclient.ActiveContractsAPIDisplay
	err = client.Call(&contracts, "sclient_contracts")
	if err != nil {
		utils.Fatalf("failed to retrieve the contracts: %s", err.Error())
	}

	if len(contracts) == 0 {
		fmt.Println("Not storage contracts created yet")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ContractID", "HostID", "AbleToUpload", "AbleToRenew", "Canceled"})

	for _, contract := range contracts {
		dataEntry := []string{contract.ContractID, contract.HostID, boolToString(contract.AbleToUpload),
			boolToString(contract.AbleToRenew), boolToString(contract.Canceled)}
		table.Append(dataEntry)
	}

	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.Render()
	fmt.Println()
	return nil
}

func getFiles(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var filesInfo []storage.FileBriefInfo
	err = client.Call(&filesInfo, "clientfiles_fileList")
	if err != nil {
		utils.Fatalf("failed to get file list: %s", err.Error())
	}

	if len(filesInfo) == 0 {
		fmt.Println("No file information available yet, you must upload a file first")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Path", "Status", "UploadProgress"})

	for _, fileInfo := range filesInfo {
		dataEntry := []string{fileInfo.Path, fileInfo.Status, floatToString(fileInfo.UploadProgress)}
		table.Append(dataEntry)
	}

	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.Render()
	fmt.Println()

	return nil
}

func getContract(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var id string
	if !ctx.IsSet(contractIDFlag.Name) {
		utils.Fatalf("the --contractid flag must be used to specify which contract information want to be retrieved")
	} else {
		id = ctx.String(contractIDFlag.Name)
	}

	var contract storageclient.ContractMetaDataAPIDisplay
	err = client.Call(&contract, "sclient_contract", id)
	if err != nil {
		utils.Fatalf("failed to retrieve detailed contract information: %s", err.Error())
	}

	fmt.Printf(`Contract Information:
	ContractID:           %s
	HostID:               %v
	Balance:              %s
	UploadCost:           %s
	DownloadCost:         %s
	StorageCost:          %s
	GasCost:              %s
	ContractCost:         %s
	TotalCost:            %s
	ContractStart:        %s
	ContractEnd:          %s
	UploadAbility:        %s
	RenewAbility:         %s
	Canceled:             %s

Latest ContractRevision Information:
	ParentID:                    %v
	UnlockConditions:            %v
	NewRevisionNumber:           %v
	NewFileSize:                 %v
	NewFileMerkleRoot:           %v
	NewWindowStart:              %v
	NewWindowEnd:                %v
	NewValidProofOutputs:        %v
	NewMissedProofOutputs        %v
`, contract.ID, contract.EnodeID, contract.ContractBalance, contract.UploadCost, contract.DownloadCost,
		contract.StorageCost, contract.GasCost, contract.ContractFee, contract.TotalCost, contract.StartHeight,
		contract.EndHeight, contract.UploadAbility, contract.RenewAbility, contract.Canceled,
		contract.LatestContractRevision.ParentID, contract.LatestContractRevision.UnlockConditions,
		contract.LatestContractRevision.NewRevisionNumber, contract.LatestContractRevision.NewFileSize,
		contract.LatestContractRevision.NewFileMerkleRoot, contract.LatestContractRevision.NewWindowStart,
		contract.LatestContractRevision.NewWindowEnd, contract.LatestContractRevision.NewValidProofOutputs,
		contract.LatestContractRevision.NewMissedProofOutputs)

	return nil
}

func getPaymentAddress(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var address common.Address
	err = client.Call(&address, "sclient_paymentAddress")
	if err != nil {
		utils.Fatalf("failed to retrieve the payment address used for storage service: %s", err.Error())
	}

	fmt.Println("Payment Address:", address.String())
	return nil
}

func setPaymentAddress(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var address string
	if !ctx.IsSet(paymentAddressFlag.Name) {
		utils.Fatalf("the --address flag must be used to specify which account address want to be used for storage service")
	} else {
		address = ctx.String(paymentAddressFlag.Name)
	}

	var result bool
	if err = client.Call(&result, "sclient_setPaymentAddress", address); err != nil {
		utils.Fatalf("failed to set the payment address for storage service: %s", err.Error())
	}

	if !result {
		fmt.Println("failed to set up the payment address, you must set up an account owned by your local wallet")
		return nil
	}

	fmt.Println("the payment address for storage service has been successfully set to: ", address)
	return nil
}

func setClientConfig(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var settings = make(map[string]string)

	if ctx.IsSet(contractPeriodFlag.Name) {
		settings["period"] = ctx.String(contractPeriodFlag.Name)
	}

	if ctx.IsSet(contractHostFlag.Name) {
		settings["hosts"] = ctx.String(contractHostFlag.Name)
	}

	if ctx.IsSet(contractFundFlag.Name) {
		settings["fund"] = ctx.String(contractFundFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "sclient_setConfig", settings); err != nil {
		utils.Fatalf("%s", err.Error())
	}

	fmt.Println(resp)
	return nil
}

func fileUpload(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var source, destination string
	if !ctx.IsSet(fileSourceFlag.Name) {
		utils.Fatalf("must specify the source path of the file used for uploading")
	} else {
		source = ctx.String(fileSourceFlag.Name)
	}

	if !ctx.IsSet(fileDestinationFlag.Name) {
		utils.Fatalf("must specify the destination path used for saving the file")
	} else {
		destination = ctx.String(fileDestinationFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "sclient_upload", source, destination); err != nil {
		utils.Fatalf("failed to upload the file: %s", err.Error())
	}

	fmt.Println("File uploaded successfully")
	return nil
}

// download remote file by sync mode
// NOTE: RPC not support async download, because it is stateless, should block until download task done.
func fileDownload(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var source, destination string
	if !ctx.IsSet(fileSourceFlag.Name) {
		utils.Fatalf("must specify the source path of the file used for uploading")
	} else {
		source = ctx.String(fileSourceFlag.Name)
	}

	if !ctx.IsSet(fileDestinationFlag.Name) {
		utils.Fatalf("must specify the destination path used for saving the file")
	} else {
		destination = ctx.String(fileDestinationFlag.Name)
	}

	var result string
	err = client.Call(&result, "sclient_downloadSync", source, destination)
	if err != nil {
		utils.Fatalf("failed to download the file: %s", err.Error())
	}

	fmt.Println(result)
	return nil
}

func getFile(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var filePath string
	if !ctx.IsSet(filePathFlag.Name) {
		utils.Fatalf("must specify the file path used for uploading in order to get the detailed file information")
	} else {
		filePath = ctx.String(filePathFlag.Name)
	}

	var fileInfo storage.FileInfo
	if err = client.Call(&fileInfo, "clientfiles_detailedFileInfo", filePath); err != nil {
		utils.Fatalf("%s", err.Error())
	}

	fmt.Printf(`File Information:
	DxPath:            %s
 	Status:            %s
	SourcePath:        %s
	FileSize:          %v
	Redundancy:        %v    
	StorageOnDisk:     %v
	UploadProgress:    %v
`, fileInfo.DxPath, fileInfo.Status, fileInfo.SourcePath, fileInfo.FileSize, fileInfo.Redundancy,
		fileInfo.StoredOnDisk, fileInfo.UploadProgress)

	return nil
}

func fileRenaming(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var prevPath, newPath string
	if !ctx.IsSet(prevFilePathFlag.Name) {
		utils.Fatalf("must specify the previous file path in order to change the name")
	} else {
		prevPath = ctx.String(prevFilePathFlag.Name)
	}

	if !ctx.IsSet(newFilePathFlag.Name) {
		utils.Fatalf("must specify the new file path")
	} else {
		newPath = ctx.String(newFilePathFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "clientfiles_rename", prevPath, newPath); err != nil {
		utils.Fatalf("%s", err.Error())
	}

	fmt.Println(resp)
	return nil
}

func fileDelete(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var filePath string
	if !ctx.IsSet(filePathFlag.Name) {
		utils.Fatalf("must specify the file path used for uploading in order to get the delete the file")
	} else {
		filePath = ctx.String(filePathFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "clientfiles_delete", filePath); err != nil {
		utils.Fatalf("%s", err.Error())
	}

	fmt.Println(resp)
	return nil
}

func periodCost(ctx *cli.Context) error {
	// attaching to the remote gdx
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remove gdx, please start the gdx first: %s", err.Error())
	}

	// getting the storage period cost
	var cost storage.PeriodCost
	if err = client.Call(&cost, "sclient_periodCost"); err != nil {
		utils.Fatalf("failed to get the client's period cost: %s", err.Error())
	}

	// print out the result
	fmt.Printf(`Period Cost:
	ContractFees:                 %v camel
 	UploadCost:                   %v camel
	DownloadCost:                 %v camel
	StorageCost:                  %v camel
	PrevContractCost:             %v camel
	ContractFund:                 %v camel
	UnspentFund:                  %v camel
	WithheldFund:                 %v camel
	WithheldFundReleaseBlock:     %v block
`, cost.ContractFees, cost.UploadCost, cost.DownloadCost, cost.StorageCost, cost.PrevContractCost,
		cost.ContractFund, cost.UnspentFund, cost.WithheldFund, cost.WithheldFundReleaseBlock)

	return nil
}

func gdxAttach(ctx *cli.Context) (*rpc.Client, error) {
	path := node.DefaultDataDir()
	if ctx.GlobalIsSet(utils.DataDirFlag.Name) {
		path = ctx.GlobalString(utils.DataDirFlag.Name)
	}

	if path != "" {
		if ctx.GlobalBool(utils.TestnetFlag.Name) {
			path = filepath.Join(path, "testnet")
		} else if ctx.GlobalBool(utils.RinkebyFlag.Name) {
			path = filepath.Join(path, "rinkeby")
		}
	}

	endpoint := fmt.Sprintf("%s/gdx.ipc", path)

	client, err := dialRPC(endpoint)
	return client, err
}

func hostInfoTable(infos []storage.HostInfo) *tablewriter.Table {
	var formattedData [][]string

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "IP", "AcceptingStorageContracts"})

	for _, host := range infos {
		dataEntry := []string{host.EnodeID.String(), host.IP, strconv.FormatBool(host.AcceptingContracts)}
		formattedData = append(formattedData, dataEntry)
	}

	for _, data := range formattedData {
		table.Append(data)
	}

	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	return table
}

func hostRankingTable(rankings []storagehostmanager.StorageHostRank) *tablewriter.Table {
	var formattedData [][]string

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Total Evaluation", "PresenceScore", "DepositScore",
		"InteractionScore", "PriceScore", "RemainingStorageScore", "UptimeScore"})

	for _, rank := range rankings {
		dataEntry := []string{rank.EnodeID, int64ToString(rank.Evaluation), floatToString(rank.PresenceScore),
			floatToString(rank.DepositScore),
			floatToString(rank.InteractionScore), floatToString(rank.ContractPriceScore),
			floatToString(rank.StorageRemainingScore), floatToString(rank.UptimeScore)}

		formattedData = append(formattedData, dataEntry)
	}

	for _, data := range formattedData {
		table.Append(data)
	}

	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	return table
}

func floatToString(val float64) string {
	return fmt.Sprintf("%v", val)
}

func int64ToString(val int64) string {
	return fmt.Sprintf("%v", val)
}

func boolToString(val bool) string {
	if val {
		return "true"
	}
	return "false"
}
