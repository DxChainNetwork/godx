package main

import (
	"fmt"
	"github.com/DxChainNetwork/godx/cmd/utils"
	"github.com/DxChainNetwork/godx/node"
	"github.com/DxChainNetwork/godx/rpc"
	"gopkg.in/urfave/cli.v1"
	"path/filepath"
)

var storageClientCommand = cli.Command{
	Name:      "storageclient",
	Usage:     "Storage Client related operations",
	ArgsUsage: "",
	Action:    utils.MigrateFlags(getPayment),
	Category:  "Storage Client COMMANDS",
	Description: `
   	gdx storage client commands
	`,
	Subcommands: []cli.Command{
		{

			Name:        "payment",
			Usage:       "get storage client payment information",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(getPayment),
			Description: `getting storage client payment information`,
		},

		{
			Name:        "setpayment",
			Usage:       "set storage client payment information",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(setPayment),
			Description: `set storage client payment information`,
		},
	},
}

func getPayment(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to attach to remote gdx, please start the gdx first: %s", err.Error())
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
		utils.Fatalf("unable to attach to remote gdx, please start the gdx first: %s", err.Error())
	}

	var result string
	err = client.Call(&result, "storageclient_setpayment")
	if err != nil {
		utils.Fatalf("failed to configure storage client payment settings: %s", err.Error())
	}
	fmt.Println(result)
	return nil

}

func gdxAttach(ctx *cli.Context) (*rpc.Client, error) {
	endpoint := ctx.Args().First()

	if endpoint == "" {
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
		endpoint = fmt.Sprintf("%s/geth.ipc", path)
	}
	client, err := dialRPC(endpoint)
	return client, err
}
