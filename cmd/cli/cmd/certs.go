package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(certsCmd)
	certsCmd.AddCommand(certsListCmd)
}

var certsCmd = &cobra.Command{Use: "certs", Short: "Manage certificates"}

var certsListCmd = &cobra.Command{
	Use: "list", Short: "List certificates",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("GET", "/certificates?limit=100", nil)
		if err != nil {
			return err
		}
		if output == "json" {
			printJSON(data)
			return nil
		}
		var resp struct {
			Data []struct {
				ID           string `json:"id"`
				Subject      string `json:"subject"`
				SerialNumber string `json:"serial_number"`
				CertType     string `json:"cert_type"`
				ExpiresAt    string `json:"expires_at"`
			} `json:"data"`
		}
		json.Unmarshal(data, &resp)
		w := newTable()
		fmt.Fprintln(w, "ID\tSUBJECT\tSERIAL\tTYPE\tEXPIRES")
		for _, c := range resp.Data {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.ID, c.Subject, c.SerialNumber, c.CertType, c.ExpiresAt)
		}
		return w.Flush()
	},
}
