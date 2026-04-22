package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(policiesCmd)
	policiesCmd.AddCommand(policiesListCmd, policiesGetCmd, policiesCreateCmd, policiesUpdateCmd, policiesDeleteCmd, policiesAssignCmd)
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

var policiesUpdateCmd = &cobra.Command{
	Use: "update [id] [json-file]", Short: "Update policy from JSON file", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.ReadFile(args[1])
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		var body interface{}
		if err := json.Unmarshal(f, &body); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}
		data, _, err := apiRequest("PUT", "/policies/"+args[0], body)
		if err != nil {
			return err
		}
		fmt.Println("Policy updated")
		printJSON(data)
		return nil
	},
}

var policiesAssignCmd = &cobra.Command{
	Use: "assign [policy-id] [target-type] [target-id]", Short: "Assign policy to device/group/enterprise",
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{
			"target_type": args[1],
			"target_id":   args[2],
			"priority":    1,
		}
		data, _, err := apiRequest("POST", "/policies/"+args[0]+"/assignments", body)
		if err != nil {
			return err
		}
		fmt.Println("Policy assigned")
		printJSON(data)
		return nil
	},
}
