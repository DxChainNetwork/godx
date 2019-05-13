package main

import (
	"github.com/DxChainNetwork/godx/cmd/utils"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/urfave/cli.v1"
	"os"
	"strconv"
)

var hostManagerCommand = cli.Command{
	Name:      "hostmanager",
	Usage:     "Storage Host Manager related operations",
	ArgsUsage: "",
	Category:  "STORAGE HOST MANAGER COMMANDS",
	Description: `
   	gdx storage host manager commands
	`,
	Subcommands: []cli.Command{
		{
			Name:      "all",
			Usage:     "show all storage hosts learnt by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getHostInfo),
			Description: `"show all storage hosts learnt by the storage client, with some
			general information`,
		},
	},
}

func getHostInfo(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}
	var allStorageHosts []storage.HostInfo
	var formattedData [][]string

	// AllStorageHosts
	err = client.Call(&allStorageHosts, "hostmanager_allStorageHosts")
	//fmt.Println(allStorageHosts)

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID" ,"IP", "Accepting Storage Contracts"})

	for _, host := range allStorageHosts {
		dataEntry := []string{host.EnodeID.String(), host.IP, strconv.FormatBool(host.AcceptingContracts)}
		formattedData = append(formattedData, dataEntry)
	}

	for _, data := range formattedData {
		table.Append(data)
	}

	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})

	table.Render()

	return nil
}
