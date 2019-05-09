package main

import (
	"fmt"
	"github.com/DxChainNetwork/godx/cmd/utils"
	"gopkg.in/urfave/cli.v1"
)

var hostManagerTestCommand = cli.Command{
	Name:      "hmtest",
	Usage:     "Storage Host Manager related test operations",
	ArgsUsage: "",
	Category:  "Storage Host Manager COMMANDS",
	Description: `
   	gdx storage client commands
	`,
	Subcommands: []cli.Command{
		{

			Name:        "online",
			Usage:       "check if the storage client is online",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(getOnlineStatus),
			Description: `check is the storage client is online, if the 
			storage client is not connected to any peer, it should return
			false. Otherwise, true is expected`,
		},


		{
			Name:      "syncing",
			Usage:     "check if the storage client is syncing",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getSyncingStatus),
			Description: `check if the storage client is syncing, if the
			storage client is not syncing, false should be returned. Otherwise
			true is expected`,
		},

		{
			Name:      "height",
			Usage:     "get the storage host manager current syncing block height",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getBlockHeight),
			Description: `get the storage host manager current syncing block height,
			it should be equivalent to the current block height`,
		},
	},
}

func getOnlineStatus(ctx *cli.Context) error {
	client, err := GdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}
	var online bool
	err = client.Call(&online, "hostmanagerdebug_online")
	if err != nil {
		utils.Fatalf("failed to get the storage client online information: %s", err.Error())
	}
	fmt.Println(online)
	return nil
}

func getSyncingStatus(ctx *cli.Context) error {
	client, err := GdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}
	var syncing bool
	err = client.Call(&syncing, "hostmanagerdebug_syncing")
	if err != nil {
		utils.Fatalf("failed to get the storage client syncing information: %s", err.Error())
	}
	fmt.Println(syncing)
	return nil
}

func getBlockHeight(ctx *cli.Context) error {
	client, err := GdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}
	var blockHeight uint64
	err = client.Call(&blockHeight, "hostmanagerdebug_blockHeight")
	if err != nil {
		utils.Fatalf("failed to get the storage host manager block height information: %s", err.Error())
	}
	fmt.Println("Storage Host Manager Block Height: ", blockHeight)
	return nil
}
