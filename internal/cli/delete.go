package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/thinktide/tally/internal/db"
	"github.com/spf13/cobra"
)

var deleteForce bool

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

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
}

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
