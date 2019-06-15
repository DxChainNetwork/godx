package main

import (
	"fmt"
	"path/filepath"

	"github.com/DxChainNetwork/godx/cmd/utils"
	"github.com/DxChainNetwork/godx/node"
	"github.com/DxChainNetwork/godx/rpc"
	"gopkg.in/urfave/cli.v1"
)

var storageClientCommand = cli.Command{
	Name:      "storageclient",
	Usage:     "Storage Client related operations",
	ArgsUsage: "",
	Category:  "Storage Client COMMANDS",
	Description: `
   	gdx storage client commands
	`,
	Subcommands: []cli.Command{
		{

			Name:        "payment",
			Usage:       "check storage client payment information",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(getPayment),
			Description: `check storage client payment information`,
		},

		{
			Name:        "setsetting",
			Usage:       "set storage client settings",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(setClientSetting),
			Flags:       storageClientFlags,
			Description: `set storage client payment information`,
		},

		{
			Name:        "memory",
			Usage:       "check storage client memory available",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(memoryAvailable),
			Description: `check storage client memory available`,
		},

		{
			Name:        "memorylimit",
			Usage:       "check storage client memory limitation",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(memoryLimitation),
			Description: `check storage client memory limitation`,
		},

		{
			Name:      "setmemorylimit",
			Usage:     "set storage client memory limit",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(setMemoryLimit),
			Flags: []cli.Flag{
				utils.StorageClientMemoryFlag,
			},
			Description: `set storage client memory limit`,
		},

		{
			Name:      "download",
			Usage:     "download file by sync mode",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(downloadSync),
			Flags: []cli.Flag{
				utils.DownloadLengthFlag,
				utils.DownloadOffsetFlag,
				utils.DownloadLocalPathFlag,
				utils.DownloadRemotePathFlag,
			},
			Description: `download file by sync mode`,
		},
	},
}

func getPayment(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var result string
	err = client.Call(&result, "storageclient_payment")
	if err != nil {
		utils.Fatalf("failed to get storage client payment information: %s", err.Error())
	}

	fmt.Println(result)
	return nil
}

func setClientSetting(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	// check if the flag is set
	if !ctx.GlobalIsSet(utils.PeriodFlag.Name) {
		utils.Fatalf("please use --memorylimit flag to specify the memory limitation")
	}

	// get the value from the flag
	limit := ctx.GlobalUint64(utils.StorageClientMemoryFlag.Name)
	if limit <= 0 {
		utils.Fatalf("memory limitation must be greater than 0")
	}

	var response string
	err = client.Call(&response, "storageclient_setClientSetting")
	if err != nil {
		utils.Fatalf("failed to configure storage client payment settings: %s", err.Error())
	}
	fmt.Println(response)
	return nil
}

func memoryAvailable(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var memory uint64
	err = client.Call(&memory, "storageclient_memoryAvailable")
	if err != nil {
		utils.Fatalf("failed to get current available memory: %s", err.Error())
	}
	fmt.Printf("Storage Client Memory Available: %d B \n", memory)
	return nil
}

func memoryLimitation(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}
	var limit uint64
	err = client.Call(&limit, "storageclient_memoryLimit")
	if err != nil {
		utils.Fatalf("failed to get memory limitation: %s", err.Error())
	}
	fmt.Printf("Storage Client Memory Limit: %d B \n", limit)
	return nil
}

func setMemoryLimit(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	// check if the flag is set
	if !ctx.GlobalIsSet(utils.StorageClientMemoryFlag.Name) {
		utils.Fatalf("please use --memorylimit flag to specify the memory limitation")
	}

	// get the value from the flag
	limit := ctx.GlobalUint64(utils.StorageClientMemoryFlag.Name)
	if limit <= 0 {
		utils.Fatalf("memory limitation must be greater than 0")
	}

	var resp string
	err = client.Call(&resp, "storageclient_setMemoryLimit", limit)
	if err != nil {
		utils.Fatalf("failed to set storage client memory limitation: %s", err)
	}

	fmt.Println(resp)
	return nil
}

// download remote file by sync mode
//
// NOTE: RPC not support async download, because it is stateless, should block until download task done.
func downloadSync(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	if !ctx.GlobalIsSet(utils.DownloadLengthFlag.Name) {
		utils.Fatalf("please use --length flag to specify the download length")
	}

	if !ctx.GlobalIsSet(utils.DownloadOffsetFlag.Name) {
		utils.Fatalf("please use --offset flag to specify the download offset")
	}

	if !ctx.GlobalIsSet(utils.DownloadLocalPathFlag.Name) {
		utils.Fatalf("please use --localpath flag to specify the download localpath")
	}

	if !ctx.GlobalIsSet(utils.DownloadRemotePathFlag.Name) {
		utils.Fatalf("please use --remotepath flag to specify the download remotepath")
	}

	length := ctx.GlobalUint64(utils.DownloadLengthFlag.Name)
	if length <= 0 {
		utils.Fatalf("download length must be greater than 0")
	}

	offset := ctx.GlobalUint64(utils.DownloadOffsetFlag.Name)
	if offset < 0 {
		utils.Fatalf("download offset can not be negative")
	}

	localpath := ctx.GlobalString(utils.DownloadLocalPathFlag.Name)
	if localpath == "" {
		utils.Fatalf("download localpath can not be empty")
	}

	remotepath := ctx.GlobalString(utils.DownloadRemotePathFlag.Name)
	if remotepath == "" {
		utils.Fatalf("download remotepath can not be empty")
	}

	var result string
	err = client.Call(&result, "storageclient_downloadSync", length, offset, localpath, remotepath)
	if err != nil {
		utils.Fatalf("failed to download by sync mode: %s", err.Error())
	}
	fmt.Println(result)
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

	endpoint := fmt.Sprintf("%s/geth.ipc", path)

	client, err := dialRPC(endpoint)
	return client, err
}
