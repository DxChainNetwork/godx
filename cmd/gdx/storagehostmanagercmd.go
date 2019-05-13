package main

import (
	"fmt"
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
			Action:    utils.MigrateFlags(getAllHostInfo),
			Description: `"show all storage hosts learnt by the storage client, with some
			general information`,
		},
		{
			Name:      "active",
			Usage:     "show active storage hosts learnt by the storage client",
			ArgsUsage: "",
			Action:    utils.MigrateFlags(getActiveHost),
			Description: `"show active storage hosts learnt by the storage client, with some
			general information`,
		},
		{
			Name:      "retrieve",
			Usage:     "show active storage hosts learnt by the storage client",
			ArgsUsage: "",
			Flags: []cli.Flag{
				utils.StorageHostManagerEnodeFlag,
			},
			Action: utils.MigrateFlags(getHostInfo),
			Description: `"show active storage hosts learnt by the storage client, with some
			general information`,
		},
	},
}

func getAllHostInfo(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var allStorageHosts []storage.HostInfo

	// AllStorageHosts
	err = client.Call(&allStorageHosts, "hostmanager_allStorageHosts")
	if err != nil {
		utils.Fatalf("unable to get all storage host information: %s", err.Error())
	}

	if len(allStorageHosts) == 0 {
		fmt.Println("No storage hosts can be found")
		return nil
	}

	table := hostInfoTable(allStorageHosts)
	table.Render()

	return nil
}

func getActiveHost(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var activeHosts []storage.HostInfo

	// Active Storage Hosts
	err = client.Call(&activeHosts, "hostmanager_activeStorageHosts")
	if err != nil {
		utils.Fatalf("unable to get all active storage host information: %s", err.Error())
	}

	if len(activeHosts) == 0 {
		fmt.Println("No active storage hosts can be found")
		return nil
	}

	table := hostInfoTable(activeHosts)
	table.Render()
	return nil
}

func getHostInfo(ctx *cli.Context) error {
	var id string
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	if !ctx.GlobalIsSet(utils.StorageHostManagerEnodeFlag.Name) {
		utils.Fatalf("the --enodeid flag must be used to specify which storage host information want to be retrieved")
	} else {
		id = ctx.GlobalString(utils.StorageHostManagerEnodeFlag.Name)
	}

	var info storage.HostInfo

	// Specific Host Information
	err = client.Call(&info, "hostmanager_storageHost", id)
	if err != nil {
		utils.Fatalf("failed to retrieve the storage host information: %s", err.Error())
	}

	if info.IP == "" {
		fmt.Println("the enode ID you entered does not exist")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "IP", "AcceptingStorageContracts", "RemainingStorage",
		"Deposit", "Contract Price", "DownloadBandwidthPrice", "Storage Price", "UploadBandwidthPrice",
		"SectorAccessPrice"})
	dataEntry := []string{info.EnodeID.String(), info.IP, strconv.FormatBool(info.AcceptingContracts),
		strconv.FormatUint(info.RemainingStorage, 10), info.Deposit.String(), info.ContractPrice.String(), info.DownloadBandwidthPrice.String(),
		info.StoragePrice.String(), info.UploadBandwidthPrice.String(), info.SectorAccessPrice.String()}

	table.Append(dataEntry)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.Render()
	return nil
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
