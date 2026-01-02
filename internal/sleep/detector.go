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
	// System sleep: 2026-01-01 18:40:56 -0500 Sleep               	Entering Sleep state
	// System wake:  2026-01-01 18:44:41 -0500 Wake                	Wake from Deep Idle
	// Display off:  2026-01-01 08:22:58 -0500 Notification        	Display is turned off
	// Display on:   2026-01-01 08:25:25 -0500 Notification        	Display is turned on

	systemSleepPattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) [+-]\d{4}\s+Sleep\s+Entering Sleep state`)
	systemWakePattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) [+-]\d{4}\s+Wake\s+(Wake from|DarkWake to FullWake)`)
	displayOffPattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) [+-]\d{4}\s+Notification\s+Display is turned off`)
	displayOnPattern := regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) [+-]\d{4}\s+Notification\s+Display is turned on`)

	type event struct {
		time    time.Time
		isStart bool // true = sleep/display off, false = wake/display on
	}

	var events []event

	lines := strings.Split(log, "\n")
	for _, line := range lines {
		var t time.Time
		var isStart bool
		var matched bool

		if match := systemSleepPattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = true
			matched = true
		} else if match := systemWakePattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = false
			matched = true
		} else if match := displayOffPattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = true
			matched = true
		} else if match := displayOnPattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = false
			matched = true
		}

		if matched && t.After(since) {
			events = append(events, event{time: t, isStart: isStart})
		}
	}

	// Build sleep periods from events
	// We need to pair off -> on events, handling overlapping system/display sleep
	var periods []SleepPeriod
	var sleepStart *time.Time

	for _, e := range events {
		if e.isStart {
			if sleepStart == nil {
				sleepStart = &e.time
			}
		} else {
			if sleepStart != nil {
				// Only count if sleep was more than 1 minute (ignore brief display flickers)
				duration := e.time.Sub(*sleepStart)
				if duration >= time.Minute {
					periods = append(periods, SleepPeriod{
						SleepTime: *sleepStart,
						WakeTime:  e.time,
					})
				}
				sleepStart = nil
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
