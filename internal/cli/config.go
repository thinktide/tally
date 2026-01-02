package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/config"
)

// configCmd is a command for managing application configuration settings.
//
// It provides subcommands to list all settings, retrieve specific values, and modify configuration options.
// Configuration settings include properties like `output.format` and `data.location`.
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

// configListCmd is a Cobra command that lists all configuration settings currently available in the application.
//
// It retrieves both default and user-configured settings, formats them into a table, and displays them in the console.
//
// The command interacts with [config.List] to gather all configuration data and outputs a structured view of key-value pairs.
// If any error occurs during the retrieval process, an appropriate error message is returned to the user.
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration settings",
	RunE:  runConfigList,
}

// configGetCmd defines a command to retrieve a configuration value by its key. It requires a single key as an argument.
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

// configSetCmd is a command that allows users to set a specific configuration key to a defined value.
//
// The command requires two arguments: a key and a value. It ensures the provided key is valid by checking
// against the supported configuration keys. For selected keys, it further validates the provided value.
//
// If the key is recognized, and the value passes validation, it updates the configuration and prints the
// newly set key-value pair.
//
// Errors include:
//   - The key is unrecognized.
//   - The value for a known key does not match expected constraints.
//   - An issue occurs while saving the configuration.
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

// init initializes and registers subcommands for the `config` command.
//
// It adds [configListCmd], [configGetCmd], and [configSetCmd] as subcommands to [configCmd].
// These subcommands enable listing, retrieving, and setting configuration values, respectively.
func init() {
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}

// runConfigList lists all configuration settings and displays them in a tabular format.
//
// This function retrieves the current configuration by merging stored settings with default values.
// The settings are then formatted into a table and printed to stdout for user visibility.
//
//	cmd: Represents the executed [cobra.Command] associated with this function.
//	args: Contains arguments passed to the command, although they are not used in this function.
//
// Returns an error if the configuration settings fail to load or there is an issue rendering the output.
//
// The displayed table includes columns:
//   - "Key": The name of the configuration setting.
//   - "Value": The current value for the configuration key.
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

// runConfigGet retrieves the value of a configuration key provided as an argument.
//
// The function validates that the given key exists using [config.IsValidKey]. If the key is invalid,
// it returns an error with a list of valid keys. Otherwise, it attempts to retrieve the value
// associated with the key using [config.Get]. If retrieving the value fails, the function returns
// an error describing the failure.
//
// The retrieved value is printed to the standard output upon success.
//
//   - cmd: Command that triggered the invocation.
//   - args: Arguments passed to the command, where the first argument is the configuration key.
//
// Returns an error if the key is invalid, if retrieving the configuration value fails,
// or if any other runtime issue is encountered.
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

// runConfigSet updates a configuration key with a new value.
//
// The function ensures the key is valid and, for specific keys like
// [config.KeyOutputFormat], validates the value against predefined options.
// Invalid keys or values will result in errors.
//
// If the key and value are valid, the configuration is updated using [config.Set].
//
//   - cmd: The [cobra.Command] instance representing the executed command.
//   - args: A slice where args[0] is the key to update and args[1] is the new value.
//
// Returns an error if the key is not valid, the value is invalid, or the
// configuration update fails in [config.Set].
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
