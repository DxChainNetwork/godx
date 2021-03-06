// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file

package main

import (
	"fmt"
	"github.com/DxChainNetwork/godx/cmd/utils"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storagehost"

	"gopkg.in/urfave/cli.v1"
)

var (
	acceptingContractsFlag = cli.StringFlag{
		Name:  "acceptingContracts",
		Usage: "BOOL - whether the host accepts new contracts",
	}

	maxDepositFlag = cli.StringFlag{
		Name:  "maxDeposit",
		Usage: "CURRENCY - the max deposit for for a single contract",
	}

	budgetPriceFlag = cli.StringFlag{
		Name:  "depositBudget",
		Usage: "CURRENCY - the maximum deposit for all contracts",
	}

	storagePriceFlag = cli.StringFlag{
		Name:  "storagePrice",
		Usage: "CURRENCY - the storage price per block per byte",
	}

	uploadPriceFlag = cli.StringFlag{
		Name:  "uploadPrice",
		Usage: "CURRENCY - upload bandwidth price per byte",
	}

	downloadPriceFlag = cli.StringFlag{
		Name:  "downloadPrice",
		Usage: "CURRENCY - download bandwidth price per byte",
	}

	contractPriceFlag = cli.StringFlag{
		Name:  "contractPrice",
		Usage: "CURRENCY - the contract price when creating the contract",
	}

	depositPriceFlag = cli.StringFlag{
		Name:  "deposit",
		Usage: "CURRENCY - deposit price per block per byte",
	}

	storageDurationFlag = cli.StringFlag{
		Name:  "maxDuration",
		Usage: "DURATION - the max duration for a storage contract",
	}

	hostPaymentAddressFlag = cli.StringFlag{
		Name:  "address",
		Usage: "Payment address for the storage service",
	}

	folderSizeFlag = cli.StringFlag{
		Name:  "size",
		Usage: "Size of the folder",
	}

	folderPathFlag = cli.StringFlag{
		Name:  "folderPath",
		Usage: "Path of the folder",
	}
)

var storageHostCommand = cli.Command{
	Name:      "shost",
	Usage:     "Storage host related operations",
	ArgsUsage: "",
	Category:  "STORAGE HOST COMMANDS",
	Description: `
	gdx storage host commands`,
	Subcommands: []cli.Command{
		{
			Name:      "config",
			Usage:     "Retrieve storage host configurations",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getHostConfig),
			Description: `
			gdx shost config

will display the current storage host settings used for the storage service. Including
but not limited to the contract price, upload bandwidth price, download bandwidth price,
and etc.`,
		},

		{
			Name:      "setConfig",
			Usage:     "Set the storage host configurations",
			ArgsUsage: "",
			Flags: []cli.Flag{
				acceptingContractsFlag,
				storageDurationFlag,
				depositPriceFlag,
				contractPriceFlag,
				downloadPriceFlag,
				uploadPriceFlag,
				storagePriceFlag,
				budgetPriceFlag,
				maxDepositFlag,
			},

			Action: utils.MigrateFlags(setHostConfig),
			Description: `
			gdx shost setConfig [--acceptingContracts arg] [--maxDeposit arg] [--depositBudget arg] [--storagePrice arg] [--uploadPrice arg] [--downloadPrice arg] [--contractPrice arg] [--deposit arg] [--maxDuration arg]

change the storage host configuration. The parameters include but not limited to 
acceptingContracts, storagePrice, uploadPrice, downloadPrice, etc. A complete set of 
configurable parameters please read the list of flags.

The values are associated with units.
	BOOL:       {"true", "false"}
	CURRENCY:   {"camel", "gcamel", "dx"}
	DURATION:   {"h", "b", "d", "w", "m", "y"}`,
		},
		{
			Name:      "setPaymentAddr",
			Usage:     "Register the account address to be used for the storage services",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setHostPaymentAddress),
			Flags: []cli.Flag{
				hostPaymentAddressFlag,
			},
			Description: `
			gdx shost setPaymentAddr [--address arg]
is used to register the account address to be used for the storage services. Deposit and money spent for host
announcement will be deducted from this account. Moreover, the profit getting from saving files for storage
client will be saved into this address as well.`,
		},

		{
			Name:      "folders",
			Usage:     "Retrieve the information of folders created for storing data uploaded by client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getHostFolders),
			Description: `
			gdx shost folders

will display the detailed information of the folders created by the storage host for
storing data uploaded by the storage client, including the folder's absolute path,
total amount of data it can hold, and how many data it received in term of sector.`,
		},

		{
			Name:      "finance",
			Usage:     "Retrieve detailed financial metrics for hosting the storage service, including revenue and lost",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getFinance),
			Description: `
			gdx shost finance

will display the detailed financial metrics for hosting the storage service, including both revenue, lost, and
potential revenue.`,
		},

		{
			Name:      "announce",
			Usage:     "Announce the node as a storage host node",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(makeAnnounce),
			Description: `
			gdx shost announce

will announce the node as a storage host node. By announcing a node as a storage host, storage client
node will automatically communicate with it, get its settings, and to determine if it is the best fit.
If the host node has higher evaluation, client will automatically create contract with it.
		`,
		},

		{
			Name:      "addFolder",
			Usage:     "Allocate disk space for saving data uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(addFolder),
			Flags: []cli.Flag{
				folderPathFlag,
				folderSizeFlag,
			},
			Description: `
			gdx shost addFolder [--folderPath arg] [--size arg]

will allocate disk space for saving data uploaded by storage client. Physical folder will be created
under the file path specified using --folderPath. The size must be specified using --size as well.

Here are some supported folder size unit (NOTE: the unit must be specified as well):
	{"kb", "mb", "gb", "tb", "kib", "mib", "gib", "tib"}`,
		},

		{
			Name:      "resizeFolder",
			Usage:     "Resize the disk space allocated for saving data uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(resizeFolder),
			Flags: []cli.Flag{
				folderPathFlag,
				folderSizeFlag,
			},
			Description: `
			gdx shost resize [--folderPath arg] [--size arg]

will resize the disk space allocated for saving data uploaded by the storage client. The usage of this
command is similar to addFolder, where folder size and folder path must be explicitly specified using the 
flag --folderPath and --size`,
		},

		{
			Name:      "deleteFolder",
			Usage:     "Free up the disk space used for saving data uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(deleteFolder),
			Flags: []cli.Flag{
				folderPathFlag,
			},
			Description: `
			gdx shost deleteFolder [--folderPath arg]

will free up the disk space used for saving data uploaded by the storage client. The folder path must be
specified using --folderPath.`,
		},

		{
			Name:      "paymentAddr",
			Usage:     "Retrieve the account address used for storage service revenue",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getHostPaymentAddress),
			Description: `
			gdx shost paymentAddr

will display the account address used for the storage service. Unless user set it explicitly, the payment
address will always be the first account address`,
		},
	},
}

func getHostConfig(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var config storage.HostIntConfigForDisplay
	if err = client.Call(&config, "shost_getHostConfig"); err != nil {
		utils.Fatalf("failed to get the storage host configuration: %s", err.Error())
	}

	fmt.Printf(`Host Configuration:
	AcceptingContracts:            %v
	MaxDownloadBatchSize:          %v
	MaxDuration:                   %v
	MaxReviseBatchSize:            %v
	WindowSize:                    %v
	PaymentAddress:                %s 
	Deposit:                       %v
	DepositBudget:                 %v
	MaxDeposit:                    %v
	BaseRPCPrice:                  %v
	ContractPrice:                 %v
	DownloadBandwidthPrice:        %v
	SectorAccessPrice:             %v
	StoragePrice:                  %v
	UploadBandwidthPrice:          %v
`, config.AcceptingContracts, config.MaxDownloadBatchSize, config.MaxDuration,
		config.MaxReviseBatchSize, config.WindowSize, config.PaymentAddress,
		config.Deposit, config.DepositBudget, config.MaxDeposit, config.BaseRPCPrice,
		config.ContractPrice, config.DownloadBandwidthPrice, config.SectorAccessPrice,
		config.StoragePrice, config.UploadBandwidthPrice)

	return nil
}

// setClientConfig set the storage host settings
func setHostConfig(ctx *cli.Context) error {
	// setConfig is attached to the backend
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}
	// get the config from user set flags
	config := hostConfigFromFlags(ctx)
	// set the host config
	var resp string
	if err = client.Call(&resp, "shost_setConfig", config); err != nil {
		utils.Fatalf("failed to set config: %v", err)
	}
	fmt.Printf("%v\n", resp)
	return nil
}

// hostConfigFromFlags gets the user set flag values from the command line arguments
func hostConfigFromFlags(ctx *cli.Context) map[string]string {
	config := make(map[string]string)

	// set the value of accepting contracts
	if ctx.IsSet(acceptingContractsFlag.Name) {
		acceptingContracts := ctx.String(acceptingContractsFlag.Name)
		config["acceptingContracts"] = acceptingContracts
	}
	// set the value of max deposit
	if ctx.IsSet(maxDepositFlag.Name) {
		maxDeposit := ctx.String(maxDepositFlag.Name)
		config["maxDeposit"] = maxDeposit
	}
	// set the value of budget price
	if ctx.IsSet(budgetPriceFlag.Name) {
		budget := ctx.String(budgetPriceFlag.Name)
		config["depositBudget"] = budget
	}
	// set the value of storage price
	if ctx.IsSet(storagePriceFlag.Name) {
		storagePrice := ctx.String(storagePriceFlag.Name)
		config["storagePrice"] = storagePrice
	}
	// set the upload price
	if ctx.IsSet(uploadPriceFlag.Name) {
		uploadPrice := ctx.String(uploadPriceFlag.Name)
		config["uploadBandwidthPrice"] = uploadPrice
	}
	// set the download price
	if ctx.IsSet(downloadPriceFlag.Name) {
		downloadPrice := ctx.String(downloadPriceFlag.Name)
		config["downloadBandwidthPrice"] = downloadPrice
	}
	// set the contract price
	if ctx.IsSet(contractPriceFlag.Name) {
		contractPrice := ctx.String(contractPriceFlag.Name)
		config["contractPrice"] = contractPrice
	}
	// set the deposit price
	if ctx.IsSet(depositPriceFlag.Name) {
		deposit := ctx.String(depositPriceFlag.Name)
		config["deposit"] = deposit
	}
	// set the duration
	if ctx.IsSet(storageDurationFlag.Name) {
		maxDuration := ctx.String(storageDurationFlag.Name)
		config["maxDuration"] = maxDuration
	}

	return config
}

func setHostPaymentAddress(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var address string
	if !ctx.IsSet(paymentAddressFlag.Name) {
		utils.Fatalf("the --address flag must be used to specify which account address want to be used")
	} else {
		address = ctx.String(paymentAddressFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setPaymentAddress", address); err != nil {
		utils.Fatalf("failed to set up the payment address: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func getHostFolders(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var hostFolders []storage.HostFolder
	if err = client.Call(&hostFolders, "shost_folders"); err != nil {
		utils.Fatalf("failed to get the information of those folders: %s", err.Error())
	}

	if len(hostFolders) == 0 {
		fmt.Println("No folders created, please use `addfolder` command to create the folder first")
		return nil
	}

	fmt.Println("Folders Count: ", len(hostFolders))
	for i, folder := range hostFolders {
		fmt.Printf(`Host Folder #%v:
	Folder Path:    %s
	TotalSpace:     %v sectors
	UsedSpace:      %v sectors
`, i+1, folder.Path, folder.TotalSectors, folder.UsedSectors)
	}

	return nil
}

func getFinance(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var finance storagehost.HostFinancialMetricsForDisplay
	if err = client.Call(&finance, "shost_getFinancialMetrics"); err != nil {
		utils.Fatalf("failed to get the host financial metrics: %s", err.Error())
	}

	fmt.Printf(`Host Financial Metrics:
	ContractCount:                          %v
	ContractCompensation:                   %v 
	PotentialContractCompensation:          %v 
	LockedStorageDeposit:                   %v 
	LostRevenue:                            %v 
	LostStorageDeposit:                     %v 
	PotentialStorageRevenue:                %v 
	RiskedStorageDeposit:                   %v 
	StorageRevenue:                         %v 
	TransactionFeeExpenses:                 %v 
	DownloadBandwidthRevenue:               %v 
	PotentialDownloadBandwidthRevenue:      %v 
	PotentialUploadBandwidthRevenue:        %v 
	UploadBandwidthRevenue:                 %v 
`, finance.ContractCount, finance.ContractCompensation, finance.PotentialContractCompensation,
		finance.LockedStorageDeposit, finance.LostRevenue, finance.LostStorageDeposit, finance.PotentialStorageRevenue,
		finance.RiskedStorageDeposit, finance.StorageRevenue, finance.TransactionFeeExpenses, finance.DownloadBandwidthRevenue,
		finance.PotentialDownloadBandwidthRevenue, finance.PotentialUploadBandwidthRevenue, finance.UploadBandwidthRevenue)

	return nil
}

func makeAnnounce(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var resp string
	if err = client.Call(&resp, "shost_announce"); err != nil {
		utils.Fatalf("failed to announce the node as a storage host: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func addFolder(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var path, size string
	if !ctx.IsSet(folderPathFlag.Name) {
		utils.Fatalf("the --folderpath flag must be used to specify the folder creation location")
	} else {
		path = ctx.String(folderPathFlag.Name)
	}

	if !ctx.IsSet(folderSizeFlag.Name) {
		utils.Fatalf("the --size flag must be used to specify the folder creation size")
	} else {
		size = ctx.String(folderSizeFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_addStorageFolder", path, size); err != nil {
		utils.Fatalf("failed to add folder: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func resizeFolder(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var path, size string
	if !ctx.IsSet(folderPathFlag.Name) {
		utils.Fatalf("the --folderpath flag must be used to specify the folder that is going to be resize")
	} else {
		path = ctx.String(folderPathFlag.Name)
	}

	if !ctx.IsSet(folderSizeFlag.Name) {
		utils.Fatalf("the --size flag must be used to specify the folder size")
	} else {
		size = ctx.String(folderSizeFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_resizeFolder", path, size); err != nil {
		utils.Fatalf("failed to resize the folder: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func deleteFolder(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var path string
	if !ctx.IsSet(folderPathFlag.Name) {
		utils.Fatalf("the --folderpath flag must be used to specify the folder to be deleted")
	} else {
		path = ctx.String(folderPathFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_deleteFolder", path); err != nil {
		utils.Fatalf("error deleting the folder: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func getHostPaymentAddress(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var resp string
	err = client.Call(&resp, "shost_getPaymentAddress")
	if err != nil {
		utils.Fatalf("failed to retrieve the payment address: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}
