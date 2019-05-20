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
			Name:        "setpayment",
			Usage:       "set storage client payment information",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(setPayment),
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

func setPayment(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var result string
	err = client.Call(&result, "storageclient_setPayment")
	if err != nil {
		utils.Fatalf("failed to configure storage client payment settings: %s", err.Error())
	}
	fmt.Println(result)
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
