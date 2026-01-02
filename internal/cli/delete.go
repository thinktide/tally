package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/db"
)

// deleteForce specifies whether the delete operation should skip the user confirmation prompt.
var deleteForce bool

// deleteCmd represents the command to delete a time entry.
//
// Deletes a specific time entry when provided with an ID. Without an ID, it deletes the most recent entry.
//
// The command performs a confirmation prompt before deletion unless the --force flag is used to bypass it.
//
// Error cases include:
//   - Failure to retrieve the last entry when no ID is provided.
//   - Attempting to delete an entry that does not exist or cannot be found.
//   - Errors during the deletion process in the database.
var deleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a time entry",
	Long: `Delete a time entry. Without an ID, deletes the most recent entry.

Examples:
  tally delete                              # Delete most recent entry
  tally delete 01ABC123DEF456GHI789JKL0     # Delete specific entry
  tally delete --force                      # Skip confirmation`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDelete,
}

// init initializes the delete command's flags.
//
// This function configures the flags for the "delete" command, adding the "force" flag (`-f`) to bypass confirmation prompts.
// The "force" flag is bound to the `deleteForce` variable to control whether user confirmation is required.
func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
}

// runDelete deletes a specified time entry or the most recent one if no ID is provided.
// It retrieves the entry details, displays them to confirm the deletion, and allows the user to cancel unless forced.
//
// If no arguments are passed, runDelete will fetch the most recent entry using [db.GetLastEntry].
// If an entry ID is provided through args, it will fetch that specific entry using [db.GetEntryByID].
//
// The function prompts for confirmation before deletion unless `deleteForce` is set to true. After confirmation, it
// deletes the entry using [db.DeleteEntry] and provides feedback to indicate whether the deletion was successful.
//
//	cmd: Represents the Cobra command invoked by the user.
//	args: Contains the optional entry ID to delete.
//
// Returns an error in the following cases:
//   - Failure to fetch the last entry or specified entry.
//   - The entry ID does not exist in the database.
//   - Errors occur during entry deletion.
func runDelete(cmd *cobra.Command, args []string) error {
	var entryID string

	if len(args) == 0 {
		entry, err := db.GetLastEntry()
		if err != nil {
			return fmt.Errorf("failed to get last entry: %w", err)
		}
		if entry == nil {
			fmt.Println("No entries to delete")
			return nil
		}
		entryID = entry.ID
	} else {
		entryID = args[0]
	}

	entry, err := db.GetEntryByID(entryID)
	if err != nil {
		return fmt.Errorf("entry not found: %w", err)
	}

	// Show entry details
	fmt.Printf("Entry: %s\n", entry.ID)
	fmt.Printf("  Project: @%s\n", entry.Project.Name)
	if entry.Title != "" {
		fmt.Printf("  Title:   %s\n", entry.Title)
	}
	fmt.Printf("  Date:    %s\n", entry.StartTime.Format("2006-01-02 15:04"))
	fmt.Printf("  Duration: %s\n", formatDuration(entry.Duration()))
	fmt.Println()

	// Confirm deletion unless --force
	if !deleteForce {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Delete this entry? [y/N]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if err := db.DeleteEntry(entryID); err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	fmt.Println("Entry deleted")
	return nil
}
