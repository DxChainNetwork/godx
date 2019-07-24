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
			Name:      "addfolder",
			Usage:     "Allocate disk space for saving data uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(addFolder),
			Flags:     storageHostFlags,
			Description: `
			gdx shost addfolder --folderpath [argument] --size [argument]

will allocate disk space for saving data uploaded by storage client. Physical folder will be created
under the file path specified using --folderpath. The size must be specified using --size as well.

Here are some supported folder size unit (NOTE: the unit must be specified as well):
	{"kb", "mb", "gb", "tb", "kib", "mib", "gib", "tib"}`,
		},

		{
			Name:      "resizefolder",
			Usage:     "Resize the disk space allocated for saving data uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(resizeFolder),
			Flags:     storageHostFlags,
			Description: `
			gdx shost resize

will resize the disk space allocated for saving data uploaded by the storage client. The usage of this
command is similar to addfolder, where folder size and folder path must be explicitly specified using the 
flag --folderpath and --size`,
		},

		{
			Name:      "deletefolder",
			Usage:     "Free up the disk space used for saving data uploaded by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(deleteFolder),
			Flags:     storageHostFlags,
			Description: `
			gdx shost deletefolder --folderpath [argument]

will free up the disk space used for saving data uploaded by the storage client. The folder path must be
specified using --folderpath.`,
		},

		{
			Name:      "setduration",
			Usage:     "Specify the max time length the storage host is willing to save the data for storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setDuration),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setduration --duration [argument]

will change the storage host configuration. The duration field specified the max time length the storage
host is willing to save the files for the storage client. The --duration flag must be used to specify
the time length`,
		},

		{
			Name:      "paymentaddr",
			Usage:     "Retrieve the account address used for storage service revenue",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getHostPaymentAddress),
			Description: `
			gdx shost paymentaddr

will display the account address used for the storage service. Unless user set it explicitly, the payment
address will always be the first account address`,
		},

		{
			Name:      "setpaymentaddr",
			Usage:     "Register the account address to be used for the storage services",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setHostPaymentAddress),
			Flags: []cli.Flag{
				utils.PaymentAddressFlag,
			},
			Description: `
			gdx shost setpaymentaddr --address [parameter]

is used to register the account address to be used for the storage services. Deposit and money spent for host
announcement will be deducted from this account. Moreover, the profit getting from saving files for storage
client will be saved into this address as well.`,
		},

		{
			Name:      "setdeposit",
			Usage:     "Specifies the deposit the host is willing to put for the storage service",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setDeposit),
			Flags:     storageHostFlags,
			Description: `
			gdx shost deposit --deposit [argument]

is used to specify the deposit the host is willing to put for the storage service. If the host failed to
prove that it has the client's file at the end, a amount of money will be deducted from the deposit. Otherwise,
the deposit will be returned to host at the end of the contract.

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},

		{
			Name:      "setcontractprice",
			Usage:     "Specifies the price the client must pay for signing the contract",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setContractPrice),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setcontractprice --contractprice [argument]

is used to specify the money that storage client must be paid for singing up the contract with the storage
host. Note, the --contractprice flag must be used to specify the money. 

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},

		{
			Name:      "setdownloadbandwidthprice",
			Usage:     "Specifies the download bandwidth price the client must be paid for downloading the data",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setDownloadBandwidthPrice),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setdownloadbandwidthprice --downloadprice [argument]

is used to specify the download bandwidth price the client must be paid for download the data from the host.
It must be used along with the downloadprice flag to specify the price

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},

		{
			Name:      "setuploadbandwidthprice",
			Usage:     "Specifies the upload bandwidth price the client must be paid for uploading the data",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setUploadBandwidthPrice),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setdownloadbandwidthprice --uploadprice [argument]

is used to specify the upload bandwidth price the client must be paid for upload the data to the host.
It must be used along with the uploadprice flag to specify the price

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},

		{
			Name:      "setsectorprice",
			Usage:     "Specifies the sector price the client must be paid",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setSectorPrice),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setsectorprice --sectorprice [argument]

is used to specify the sector price that used must be paid. Data is uploaded in terms of sectors, and for
each sector the user upload or download, this price must be paid.

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},

		{
			Name:      "setstorageprice",
			Usage:     "Specifies the storage price the client must be paid",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setStoragePrice),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setstorageprice --storageprice [argument]

is used to specify the storage price that the client must be paid, it is measured in terms of /byte/block.

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},

		{
			Name:      "setbudget",
			Usage:     "Defines the maximum amount of money that the storage host can allocate as deposit",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setBudget),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setbudget --budget [argument]

is used to specify the maximum amount of money that the storage host can allocate as deposit. The host will loose
access to this amount of money within the contract period. Note: the --budget flag must be used to specify
the budget.

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},

		{
			Name:      "setmaxdeposit",
			Usage:     "Specifies the max amount of deposit the host can put into a single contract",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setMaxDeposit),
			Flags:     storageHostFlags,
			Description: `
			gdx shost setmaxdeposit --maxdeposit [argument]

is used to specify the max amount of deposit the host can put into a single contract. The difference between
this parameter and the budget parameter is that the budget parameter defines the max amount of money can
be used as deposit for all contracts that storage host signed. The --maxdeposit flag must be used to specify
the max deposit. 

Available Units: {"wei", "kwei", "mwei", "gwei", "microether", "milliether", "ether"}`,
		},
	},
}

func getHostConfig(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var config storage.HostIntConfig
	if err = client.Call(&config, "shost_getHostConfig"); err != nil {
		utils.Fatalf("failed to get the storage host configuration: %s", err.Error())
	}

	fmt.Printf(`
	
Host Configuration:
	AcceptingContracts:            %t
	MaxDownloadBatchSize:          %v bytes
	MaxDuration:                   %v blocks
	MaxReviseBatchSize:            %v bytes
	WindowSize:                    %v blocks
	PaymentAddress:                %s 
	Deposit:                       %v wei
	DepositBudget:                 %v wei
	MaxDeposit:                    %v wei
	BaseRPCPrice:               %v wei
	ContractPrice:              %v wei
	DownloadBandwidthPrice:     %v wei
	SectorAccessPrice:          %v wei
	StoragePrice:               %v wei
	UploadBandwidthPrice:       %v wei

`, config.AcceptingContracts, config.MaxDownloadBatchSize, config.MaxDuration,
		config.MaxReviseBatchSize, config.WindowSize, config.PaymentAddress.String(),
		config.Deposit, config.DepositBudget, config.MaxDeposit, config.BaseRPCPrice,
		config.ContractPrice, config.DownloadBandwidthPrice, config.SectorAccessPrice,
		config.StoragePrice, config.UploadBandwidthPrice)

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
		fmt.Printf(`

Host Folder #%v:
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

	var finance storagehost.HostFinancialMetrics
	if err = client.Call(&finance, "shost_getFinancialMetrics"); err != nil {
		utils.Fatalf("failed to get the host financial metrics: %s", err.Error())
	}

	fmt.Printf(`

Host Financial Metrics:
	ContractCount:                          %v
	ContractCompensation:                   %v wei
	PotentialContractCompensation:          %v wei
	LockedStorageDeposit:                   %v wei
	LostRevenue:                            %v wei
	LostStorageDeposit:                     %v wei
	PotentialStorageRevenue:                %v wei
	RiskedStorageDeposit:                   %v wei
	StorageRevenue:                         %v wei
	TransactionFeeExpenses:                 %v wei
	DownloadBandwidthRevenue:               %v wei
	PotentialDownloadBandwidthRevenue:      %v wei
	PotentialUploadBandwidthRevenue:        %v wei
	UploadBandwidthRevenue:                 %v wei

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
	if !ctx.GlobalIsSet(utils.FolderPathFlag.Name) {
		utils.Fatalf("the --folderpath flag must be used to specify the folder creation location")
	} else {
		path = ctx.GlobalString(utils.FolderPathFlag.Name)
	}

	if !ctx.GlobalIsSet(utils.FolderSizeFlag.Name) {
		utils.Fatalf("the --size flag must be used to specify the folder creation size")
	} else {
		size = ctx.GlobalString(utils.FolderSizeFlag.Name)
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
	if !ctx.GlobalIsSet(utils.FolderPathFlag.Name) {
		utils.Fatalf("the --folderpath flag must be used to specify the folder that is going to be resize")
	} else {
		path = ctx.GlobalString(utils.FolderPathFlag.Name)
	}

	if !ctx.GlobalIsSet(utils.FolderSizeFlag.Name) {
		utils.Fatalf("the --size flag must be used to specify the folder size")
	} else {
		size = ctx.GlobalString(utils.FolderSizeFlag.Name)
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
	if !ctx.GlobalIsSet(utils.FolderPathFlag.Name) {
		utils.Fatalf("the --folderpath flag must be used to specify the folder to be deleted")
	} else {
		path = ctx.GlobalString(utils.FolderPathFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_deleteFolder", path); err != nil {
		utils.Fatalf("error deleting the folder: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setDuration(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var duration string
	if !ctx.GlobalIsSet(utils.StorageDurationFlag.Name) {
		utils.Fatalf("the --duration flag must be used to specify the max duration")
	} else {
		duration = ctx.GlobalString(utils.StorageDurationFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setMaxDuration", duration); err != nil {
		utils.Fatalf("failed to set the max storage duration: %s", err.Error())
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

func setHostPaymentAddress(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var address string
	if !ctx.GlobalIsSet(utils.PaymentAddressFlag.Name) {
		utils.Fatalf("the --address flag must be used to specify which account address want to be used")
	} else {
		address = ctx.GlobalString(utils.PaymentAddressFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setPaymentAddress", address); err != nil {
		utils.Fatalf("failed to set up the payment address: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setDeposit(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var deposit string
	if !ctx.GlobalIsSet(utils.DepositPriceFlag.Name) {
		utils.Fatalf("the --deposity flag must be used to specify the amount of money the host is willing to be used as deposit")
	} else {
		deposit = ctx.GlobalString(utils.DepositPriceFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setDeposit", deposit); err != nil {
		utils.Fatalf("failed to set the deposit: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setContractPrice(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var contractPrice string
	if !ctx.GlobalIsSet(utils.ContractPriceFlag.Name) {
		utils.Fatalf("the --contractprice flag must be used to specify the contract price")
	} else {
		contractPrice = ctx.GlobalString(utils.ContractPriceFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setMinContractPrice", contractPrice); err != nil {
		utils.Fatalf("failed to set the contract price: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setDownloadBandwidthPrice(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var downloadPrice string
	if !ctx.GlobalIsSet(utils.DownloadPriceFlag.Name) {
		utils.Fatalf("the --downloadprice flag must be used to specify the download bandwidth price")
	} else {
		downloadPrice = ctx.GlobalString(utils.DownloadPriceFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setMinDownloadBandwidthPrice", downloadPrice); err != nil {
		utils.Fatalf("failed to set the download bandwidth price: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setUploadBandwidthPrice(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var uploadPrice string
	if !ctx.GlobalIsSet(utils.UploadPriceFlag.Name) {
		utils.Fatalf("the --uploadprice flag must be used to specify the upload bandwidth price")
	} else {
		uploadPrice = ctx.GlobalString(utils.UploadPriceFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setMinUploadBandwidthPrice", uploadPrice); err != nil {
		utils.Fatalf("failed to set the upload bandwidth price: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setSectorPrice(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var sectorPrice string
	if !ctx.GlobalIsSet(utils.SectorPriceFlag.Name) {
		utils.Fatalf("the --sectorprice flag must be used to specify the sector price")
	} else {
		sectorPrice = ctx.GlobalString(utils.SectorPriceFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setMinSectorAccessPrice", sectorPrice); err != nil {
		utils.Fatalf("failed to set the sector price: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setStoragePrice(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var storagePrice string
	if !ctx.GlobalIsSet(utils.StoragePriceFlag.Name) {
		utils.Fatalf("the --storageprice flag must be used to specify the storage price")
	} else {
		storagePrice = ctx.GlobalString(utils.StoragePriceFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setMinStoragePrice", storagePrice); err != nil {
		utils.Fatalf("failed to set the storage price: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setBudget(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var budget string
	if !ctx.GlobalIsSet(utils.BudgetPriceFlag.Name) {
		utils.Fatalf("the --budget flag must be used to specify the budget price")
	} else {
		budget = ctx.GlobalString(utils.BudgetPriceFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setDepositBudget", budget); err != nil {
		utils.Fatalf("failed to set the budget price: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}

func setMaxDeposit(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var maxDeposit string
	if !ctx.GlobalIsSet(utils.MaxDepositFlag.Name) {
		utils.Fatalf("the --maxdeposit flag must be used to specify the max deposit")
	} else {
		maxDeposit = ctx.GlobalString(utils.MaxDepositFlag.Name)
	}

	var resp string
	if err = client.Call(&resp, "shost_setMaxDeposit", maxDeposit); err != nil {
		utils.Fatalf("failed to set the max deposit: %s", err.Error())
	}

	fmt.Printf("%s \n\n", resp)
	return nil
}
