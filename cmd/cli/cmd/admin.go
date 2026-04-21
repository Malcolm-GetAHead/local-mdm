package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(usersCmd, tokensCmd, healthCmd, versionCmd)
	usersCmd.AddCommand(usersListCmd, usersCreateCmd)
	tokensCmd.AddCommand(tokensCreateCmd, tokensListCmd, tokensRevokeCmd)
}

// --- Users ---

var usersCmd = &cobra.Command{Use: "users", Short: "Manage users"}

var usersListCmd = &cobra.Command{
	Use: "list", Short: "List users",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("GET", "/users?limit=100", nil)
		if err != nil {
			return err
		}
		if output == "json" {
			printJSON(data)
			return nil
		}
		var resp struct {
			Data []struct {
				ID    string `json:"id"`
				Email string `json:"email"`
				Role  string `json:"role"`
				Active bool  `json:"is_active"`
			} `json:"data"`
		}
		json.Unmarshal(data, &resp)
		w := newTable()
		fmt.Fprintln(w, "ID\tEMAIL\tROLE\tACTIVE")
		for _, u := range resp.Data {
			fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", u.ID, u.Email, u.Role, u.Active)
		}
		return w.Flush()
	},
}

var usersCreateCmd = &cobra.Command{
	Use: "create [email] [role]", Short: "Create a user", Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]string{"email": args[0], "role": args[1]}
		data, _, err := apiRequest("POST", "/users", body)
		if err != nil {
			return err
		}
		fmt.Println("User created")
		printJSON(data)
		return nil
	},
}

// --- Tokens ---

var tokensCmd = &cobra.Command{Use: "tokens", Short: "Manage API tokens"}

var tokensCreateCmd = &cobra.Command{
	Use: "create [name]", Short: "Create an API token", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]string{"name": args[0]}
		data, _, err := apiRequest("POST", "/tokens", body)
		if err != nil {
			return err
		}
		var resp struct {
			Data struct {
				Plaintext string `json:"plaintext"`
			} `json:"data"`
		}
		json.Unmarshal(data, &resp)
		fmt.Println("Token created. Save this — it won't be shown again:")
		fmt.Println(resp.Data.Plaintext)
		return nil
	},
}

var tokensListCmd = &cobra.Command{
	Use: "list", Short: "List API tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, _, err := apiRequest("GET", "/tokens", nil)
		if err != nil {
			return err
		}
		printJSON(data)
		return nil
	},
}

var tokensRevokeCmd = &cobra.Command{
	Use: "revoke [id]", Short: "Revoke an API token", Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, _, err := apiRequest("DELETE", "/tokens/"+args[0], nil)
		if err != nil {
			return err
		}
		fmt.Println("Token revoked")
		return nil
	},
}

// --- Utility ---

var healthCmd = &cobra.Command{
	Use: "health", Short: "Check server health",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(serverURL + "/health/ready")
		if err != nil {
			return fmt.Errorf("server unreachable: %w", err)
		}
		defer resp.Body.Close()
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		if ready, ok := result["ready"].(bool); ok && ready {
			fmt.Println("✓ Server is ready")
		} else {
			fmt.Println("✗ Server is not ready")
		}
		if output == "json" {
			b, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(b))
		}
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use: "version", Short: "Show CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("localmdm-cli v1.0.0")
	},
}
