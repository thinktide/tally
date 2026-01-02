package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/thinktide/tally/internal/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `Manage tally configuration.

Examples:
  tally config list                        # List all settings
  tally config get output.format           # Get a specific setting
  tally config set output.format json      # Set a value

Available settings:
  output.format           - Default output format (table/json/csv)
  data.location           - Data directory path`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration settings",
	RunE:  runConfigList,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

func init() {
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}

func runConfigList(cmd *cobra.Command, args []string) error {
	settings, err := config.List()
	if err != nil {
		return fmt.Errorf("failed to list config: %w", err)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Key", "Value"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, key := range config.ValidKeys() {
		value := settings[key]
		table.Append([]string{key, value})
	}

	table.Render()
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	if !config.IsValidKey(key) {
		return fmt.Errorf("unknown config key: %s\nValid keys: %s",
			key, strings.Join(config.ValidKeys(), ", "))
	}

	value, err := config.Get(key)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	fmt.Println(value)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	if !config.IsValidKey(key) {
		return fmt.Errorf("unknown config key: %s\nValid keys: %s",
			key, strings.Join(config.ValidKeys(), ", "))
	}

	// Validate values for known keys
	switch key {
	case config.KeyOutputFormat:
		if value != "table" && value != "json" && value != "csv" {
			return fmt.Errorf("value must be 'table', 'json', or 'csv'")
		}
	}

	if err := config.Set(key, value); err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	fmt.Printf("%s = %s\n", key, value)
	return nil
}
