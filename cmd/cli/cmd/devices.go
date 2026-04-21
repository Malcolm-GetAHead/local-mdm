package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(devicesCmd)
	devicesCmd.AddCommand(devicesListCmd, devicesGetCmd, devicesLockCmd, devicesWipeCmd)
}

var devicesCmd = &cobra.Command{Use: "devices", Short: "Manage devices"}

var devicesListCmd = &cobra.Command{
	Use: "list", Short: "List devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("GET", "/devices?limit=100", nil)
		if err != nil {
			return err
		}
		if output == "json" {
			printJSON(data)
			return nil
		}
		var resp struct {
			Data []struct {
				ID       string `json:"id"`
				Platform string `json:"platform"`
				Name     string `json:"name"`
				Status   string `json:"status"`
			} `json:"data"`
		}
		json.Unmarshal(data, &resp)
		w := newTable()
		fmt.Fprintln(w, "ID\tPLATFORM\tNAME\tSTATUS")
		for _, d := range resp.Data {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", d.ID, d.Platform, d.Name, d.Status)
		}
		return w.Flush()
	},
}

var devicesGetCmd = &cobra.Command{
	Use: "get [id]", Short: "Get device details", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("GET", "/devices/"+args[0], nil)
		if err != nil {
			return err
		}
		printJSON(data)
		return nil
	},
}

var devicesLockCmd = &cobra.Command{
	Use: "lock [id]", Short: "Lock a device", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("POST", "/devices/"+args[0]+"/lock", nil)
		if err != nil {
			return err
		}
		fmt.Println("Device locked successfully")
		if output == "json" {
			printJSON(data)
		}
		return nil
	},
}

var devicesWipeCmd = &cobra.Command{
	Use: "wipe [id]", Short: "Wipe a device", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("POST", "/devices/"+args[0]+"/wipe", nil)
		if err != nil {
			return err
		}
		fmt.Println("Device wipe initiated")
		if output == "json" {
			printJSON(data)
		}
		return nil
	},
}
