package sleep

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
)

// SleepPeriod represents a detected sleep/wake cycle
type SleepPeriod struct {
	SleepTime time.Time
	WakeTime  time.Time
}

func (s SleepPeriod) Duration() time.Duration {
	return s.WakeTime.Sub(s.SleepTime)
}

// CheckAndHandleSleep checks for sleep events that occurred during a running timer
func CheckAndHandleSleep() error {
	// Get running entry
	entry, err := db.GetRunningEntry()
	if err != nil {
		return err
	}
	if entry == nil {
		return nil // No running timer
	}

	// Find sleep periods since the entry started
	sleepPeriods, err := getSleepPeriodsSince(entry.StartTime)
	if err != nil {
		return err
	}

	if len(sleepPeriods) == 0 {
		return nil // No sleep detected
	}

	// Filter to only unhandled sleep periods (after any existing pauses)
	var lastPauseEnd time.Time
	for _, p := range entry.Pauses {
		if p.ResumeTime != nil && p.ResumeTime.After(lastPauseEnd) {
			lastPauseEnd = *p.ResumeTime
		}
	}

	var unhandledSleep []SleepPeriod
	for _, sp := range sleepPeriods {
		if sp.WakeTime.After(lastPauseEnd) {
			unhandledSleep = append(unhandledSleep, sp)
		}
	}

	if len(unhandledSleep) == 0 {
		return nil
	}

	// Calculate total sleep time
	var totalSleep time.Duration
	for _, sp := range unhandledSleep {
		totalSleep += sp.Duration()
	}

	// Prompt user
	fmt.Printf("\nDetected computer sleep during timer: %s\n", formatDuration(totalSleep))
	for _, sp := range unhandledSleep {
		fmt.Printf("  %s - %s (%s)\n",
			sp.SleepTime.Format("15:04:05"),
			sp.WakeTime.Format("15:04:05"),
			formatDuration(sp.Duration()))
	}
	fmt.Println()

	fmt.Print("Exclude sleep time from duration? [Y/n]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" || input == "y" || input == "yes" {
		// Create pause records for each sleep period
		for _, sp := range unhandledSleep {
			if err := createPauseForSleep(entry.ID, sp); err != nil {
				return err
			}
		}
		fmt.Printf("Excluded %s of sleep time\n", formatDuration(totalSleep))
	} else {
		fmt.Println("Sleep time included in duration")
	}

	return nil
}

func getSleepPeriodsSince(since time.Time) ([]SleepPeriod, error) {
	// Run pmset to get power log
	cmd := exec.Command("pmset", "-g", "log")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get power log: %w", err)
	}

	return parsePmsetLog(string(output), since)
}

func parsePmsetLog(log string, since time.Time) ([]SleepPeriod, error) {
	// Regex patterns for sleep and wake events
	// Format: 2026-01-01 18:40:56 -0500 Sleep               	Entering Sleep state
	// Format: 2026-01-01 18:44:41 -0500 Wake                	Wake from Deep Idle
	sleepPattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) [+-]\d{4}\s+Sleep\s+Entering Sleep state`)
	wakePattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) [+-]\d{4}\s+Wake\s+(Wake from|DarkWake to FullWake)`)

	var sleepTimes []time.Time
	var wakeTimes []time.Time

	lines := strings.Split(log, "\n")
	for _, line := range lines {
		if match := sleepPattern.FindStringSubmatch(line); match != nil {
			t, err := time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			if err == nil && t.After(since) {
				sleepTimes = append(sleepTimes, t)
			}
		} else if match := wakePattern.FindStringSubmatch(line); match != nil {
			t, err := time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			if err == nil && t.After(since) {
				wakeTimes = append(wakeTimes, t)
			}
		}
	}

	// Match sleep/wake pairs
	var periods []SleepPeriod
	for _, sleepTime := range sleepTimes {
		// Find the next wake time after this sleep
		for _, wakeTime := range wakeTimes {
			if wakeTime.After(sleepTime) {
				periods = append(periods, SleepPeriod{
					SleepTime: sleepTime,
					WakeTime:  wakeTime,
				})
				break
			}
		}
	}

	return periods, nil
}

func createPauseForSleep(entryID string, sp SleepPeriod) error {
	pauseID := model.NewULID()
	_, err := db.DB.Exec(
		"INSERT INTO pauses (id, entry_id, pause_time, resume_time) VALUES (?, ?, ?, ?)",
		pauseID, entryID, sp.SleepTime, sp.WakeTime)
	return err
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	s := (d % time.Minute) / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
