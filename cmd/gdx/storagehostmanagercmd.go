package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/DxChainNetwork/godx/cmd/utils"
	"github.com/DxChainNetwork/godx/storage"
	"github.com/DxChainNetwork/godx/storage/storageclient/storagehostmanager"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/urfave/cli.v1"
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
			Usage:     "retrieve detailed information of a storage host based on the enode id learnt by the storage client",
			ArgsUsage: "",
			Flags: []cli.Flag{
				utils.StorageHostManagerEnodeFlag,
			},
			Action:      utils.MigrateFlags(getHostInfo),
			Description: `"retrieve detailed information of a storage host based on the enode id learnt by the storage client`,
		},
		{
			Name:        "ranking",
			Usage:       "display the ranking of the storage hosts learnt by the storage client",
			ArgsUsage:   "",
			Action:      utils.MigrateFlags(getRankings),
			Description: `"display the ranking of the storage hosts learnt by the storage client`,
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

func getRankings(ctx *cli.Context) error {
	client, err := gdxAttach(ctx)
	if err != nil {
		utils.Fatalf("unable to connect to remote gdx, please start the gdx first: %s", err.Error())
	}

	var rankings []storagehostmanager.StorageHostRank
	err = client.Call(&rankings, "hostmanager_storageHostRanks")
	if err != nil {
		utils.Fatalf("failed to retrieve the storage host rankings: %s", err.Error())
	}

	if len(rankings) == 0 {
		fmt.Println("No storage host can be found")
		return nil
	}

	table := hostRankingTable(rankings)
	table.Render()

	return nil
}

func hostRankingTable(rankings []storagehostmanager.StorageHostRank) *tablewriter.Table {
	var formattedData [][]string

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "Total Evaluation", "AgeFactor", "BurnFactor", "DepositFactor",
		"InteractionFactor", "PriceFactor", "RemainingStorageFactor", "UptimeFactor"})

	for _, rank := range rankings {
		dataEntry := []string{rank.EnodeID, rank.Evaluation.String(), floatToString(rank.AgeAdjustment),
			floatToString(rank.BurnAdjustment), floatToString(rank.DepositAdjustment),
			floatToString(rank.InteractionAdjustment), floatToString(rank.PriceAdjustment),
			floatToString(rank.StorageRemainingAdjustment), floatToString(rank.UptimeAdjustment)}

		formattedData = append(formattedData, dataEntry)
	}

	for _, data := range formattedData {
		table.Append(data)
	}

	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	return table
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

func floatToString(val float64) string {
	return strconv.FormatFloat(val, 'g', 1, 64)
}
