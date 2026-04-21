package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(policiesCmd)
	policiesCmd.AddCommand(policiesListCmd, policiesGetCmd, policiesCreateCmd, policiesDeleteCmd)
}

var policiesCmd = &cobra.Command{Use: "policies", Short: "Manage policies"}

var policiesListCmd = &cobra.Command{
	Use: "list", Short: "List policies",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("GET", "/policies?limit=100", nil)
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
				Name     string `json:"name"`
				Platform string `json:"platform"`
				Type     string `json:"policy_type"`
				Active   bool   `json:"is_active"`
			} `json:"data"`
		}
		json.Unmarshal(data, &resp)
		w := newTable()
		fmt.Fprintln(w, "ID\tNAME\tPLATFORM\tTYPE\tACTIVE")
		for _, p := range resp.Data {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%v\n", p.ID, p.Name, p.Platform, p.Type, p.Active)
		}
		return w.Flush()
	},
}

var policiesGetCmd = &cobra.Command{
	Use: "get [id]", Short: "Get policy details", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("GET", "/policies/"+args[0], nil)
		if err != nil {
			return err
		}
		printJSON(data)
		return nil
	},
}

var policiesCreateCmd = &cobra.Command{
	Use: "create [json-file]", Short: "Create policy from JSON file", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		var body interface{}
		if err := json.Unmarshal(f, &body); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		data, _, err := apiRequest("POST", "/policies", body)
		if err != nil {
			return err
		}
		fmt.Println("Policy created")
		printJSON(data)
		return nil
	},
}

var policiesDeleteCmd = &cobra.Command{
	Use: "delete [id]", Short: "Delete a policy", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, err := apiRequest("DELETE", "/policies/"+args[0], nil)
		if err != nil {
			return err
		}
		fmt.Println("Policy deleted")
		return nil
	},
}
