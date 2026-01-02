package sleep

import (
	"fmt"
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
	Reason    string // "Display off" or "System sleep"
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

	// Create pause records for each sleep period automatically
	var totalSleep time.Duration
	for _, sp := range unhandledSleep {
		if err := createPauseForSleep(entry.ID, sp); err != nil {
			return err
		}
		totalSleep += sp.Duration()
	}

	// Inform user about the pauses that were created
	fmt.Printf("\nDetected sleep during timer, created pause(s):\n")
	for _, sp := range unhandledSleep {
		fmt.Printf("  %s - %s (%s) [%s]\n",
			sp.SleepTime.Format("15:04:05"),
			sp.WakeTime.Format("15:04:05"),
			formatDuration(sp.Duration()),
			sp.Reason)
	}
	fmt.Printf("Total: %s (use 'tally edit' to modify)\n\n", formatDuration(totalSleep))

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
		time     time.Time
		isStart  bool   // true = sleep/display off, false = wake/display on
		isSystem bool   // true = system sleep, false = display off
	}

	var events []event

	lines := strings.Split(log, "\n")
	for _, line := range lines {
		var t time.Time
		var isStart bool
		var isSystem bool
		var matched bool

		if match := systemSleepPattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = true
			isSystem = true
			matched = true
		} else if match := systemWakePattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = false
			isSystem = true
			matched = true
		} else if match := displayOffPattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = true
			isSystem = false
			matched = true
		} else if match := displayOnPattern.FindStringSubmatch(line); match != nil {
			t, _ = time.ParseInLocation("2006-01-02 15:04:05", match[1], time.Local)
			isStart = false
			isSystem = false
			matched = true
		}

		if matched && t.After(since) {
			events = append(events, event{time: t, isStart: isStart, isSystem: isSystem})
		}
	}

	// Build sleep periods from events
	// We need to pair off -> on events, handling overlapping system/display sleep
	var periods []SleepPeriod
	var sleepStart *time.Time
	var sleepIsSystem bool

	for _, e := range events {
		if e.isStart {
			if sleepStart == nil {
				sleepStart = &e.time
				sleepIsSystem = e.isSystem
			} else if e.isSystem && !sleepIsSystem {
				// System sleep overrides display off
				sleepIsSystem = true
			}
		} else {
			if sleepStart != nil {
				// Only count if sleep was more than 1 minute (ignore brief display flickers)
				duration := e.time.Sub(*sleepStart)
				if duration >= time.Minute {
					reason := "Display off"
					if sleepIsSystem {
						reason = "System sleep"
					}
					periods = append(periods, SleepPeriod{
						SleepTime: *sleepStart,
						WakeTime:  e.time,
						Reason:    reason,
					})
				}
				sleepStart = nil
				sleepIsSystem = false
			}
		}
	}

	return periods, nil
}

func createPauseForSleep(entryID string, sp SleepPeriod) error {
	pauseID := model.NewULID()
	_, err := db.DB.Exec(
		"INSERT INTO pauses (id, entry_id, pause_time, resume_time, reason) VALUES (?, ?, ?, ?, ?)",
		pauseID, entryID, sp.SleepTime, sp.WakeTime, sp.Reason)
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
