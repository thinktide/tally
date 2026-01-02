package sleep

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thinktide/tally/internal/config"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

const inactivityThreshold = 2 * time.Minute

// CheckAndHandleInactivity checks for gaps in activity and handles them
func CheckAndHandleInactivity() error {
	// Get running entry
	entry, err := db.GetRunningEntry()
	if err != nil {
		return err
	}
	if entry == nil {
		return nil // No running timer, nothing to check
	}

	// Get last activity time
	lastActivity, err := db.GetLastActivity()
	if err != nil {
		return err
	}
	if lastActivity == nil {
		return nil // No activity recorded yet
	}

	// Calculate gap
	gap := time.Since(*lastActivity)
	if gap < inactivityThreshold {
		return nil // No significant gap
	}

	// Found a gap - handle it
	return handleInactivityGap(entry, *lastActivity, gap)
}

func handleInactivityGap(entry *model.Entry, lastActivity time.Time, gap time.Duration) error {
	// Only handle if entry was running (not already paused)
	if entry.Status != model.StatusRunning {
		return nil
	}

	fmt.Printf("\nDetected inactivity: %s\n", formatGapDuration(gap))
	fmt.Printf("  From: %s\n", lastActivity.Format("2006-01-02 15:04:05"))
	fmt.Printf("  To:   %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()

	// Check if we have a saved preference
	countSleepTime, err := config.GetBool(config.KeySleepCountSleepTime)
	if err != nil {
		return err
	}

	// Check if config has been set explicitly
	storedValue, err := config.Get(config.KeySleepCountSleepTime)
	if err != nil {
		return err
	}

	var includeTime bool
	if storedValue != "" {
		// Use stored preference
		includeTime = countSleepTime
		if includeTime {
			fmt.Println("Including inactive time in duration (based on saved preference)")
		} else {
			fmt.Println("Excluding inactive time from duration (based on saved preference)")
			// Create a pause record for the gap
			if err := createPauseForGap(entry.ID, lastActivity); err != nil {
				return err
			}
		}
	} else {
		// Ask user
		includeTime, err = askIncludeTime()
		if err != nil {
			return err
		}

		if !includeTime {
			// Create a pause record for the gap
			if err := createPauseForGap(entry.ID, lastActivity); err != nil {
				return err
			}
		}

		// Ask if they want to remember this choice
		if rememberChoice, err := askRememberChoice(); err != nil {
			return err
		} else if rememberChoice {
			if err := config.SetBool(config.KeySleepCountSleepTime, includeTime); err != nil {
				return err
			}
			fmt.Println("Preference saved. Use 'tally config set sleep.count_sleep_time <true|false>' to change.")
		}
	}

	// Check auto-resume preference
	autoResume, err := config.GetBool(config.KeySleepAutoResume)
	if err != nil {
		return err
	}

	storedAutoResume, err := config.Get(config.KeySleepAutoResume)
	if err != nil {
		return err
	}

	if storedAutoResume == "" {
		// First time - ask about auto-resume for future
		if shouldAutoResume, err := askAutoResume(); err != nil {
			return err
		} else if shouldAutoResume {
			if err := config.SetBool(config.KeySleepAutoResume, true); err != nil {
				return err
			}
			fmt.Println("Auto-resume enabled for future. Use 'tally config set sleep.auto_resume false' to disable.")
		}
	} else if autoResume {
		fmt.Println("Timer auto-resumed (based on saved preference)")
	}

	fmt.Println()
	return nil
}

func createPauseForGap(entryID string, pauseStart time.Time) error {
	// Insert a pause record with the gap
	pauseID := model.NewULID()
	now := time.Now()
	_, err := db.DB.Exec(
		"INSERT INTO pauses (id, entry_id, pause_time, resume_time) VALUES (?, ?, ?, ?)",
		pauseID, entryID, pauseStart, now)
	return err
}

func askIncludeTime() (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Include this time in duration? [y/N]: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

func askRememberChoice() (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Remember this choice for future? [y/N]: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

func askAutoResume() (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Auto-resume timer after inactivity in the future? [y/N]: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

func formatGapDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute

	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
