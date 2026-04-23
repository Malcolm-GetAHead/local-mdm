package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/malcolm-getahead/local-mdm/internal/certs"
	"github.com/spf13/cobra"
)

var (
	certPath string
	keyPath  string
)

func init() {
	rootCmd.AddCommand(certsCmd)
	certsCmd.AddCommand(certsListCmd)
	certsCmd.AddCommand(certsInitCmd)

	certsInitCmd.Flags().StringVar(&certPath, "cert", "internal/api/certs/ca.crt", "Path for CA certificate")
	certsInitCmd.Flags().StringVar(&keyPath, "key", "internal/api/certs/ca.key", "Path for CA private key")
}

var certsCmd = &cobra.Command{Use: "certs", Short: "Manage certificates"}

var certsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a new CA certificate and key",
	Long:  "Creates a new root CA certificate and private key at the specified paths. Will not overwrite existing files.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ca, err := certs.GenerateCA(certPath, keyPath)
		if err != nil {
			return fmt.Errorf("CA generation failed: %w", err)
		}
		fmt.Printf("CA certificate generated:\n")
		fmt.Printf("  Certificate: %s\n", certPath)
		fmt.Printf("  Private key: %s\n", keyPath)
		fmt.Printf("  Subject:     %s\n", ca.GetCACertificate().Subject.CommonName)
		fmt.Printf("  Expires:     %s\n", ca.GetCACertificate().NotAfter.Format("2006-01-02"))
		return nil
	},
}

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
