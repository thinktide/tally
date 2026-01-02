package service

import (
	"time"

	"github.com/jdecarlo/tally/internal/db"
	"github.com/jdecarlo/tally/internal/model"
)

type Period string

const (
	PeriodToday     Period = "today"
	PeriodYesterday Period = "yesterday"
	PeriodWeek      Period = "week"
	PeriodLastWeek  Period = "lastWeek"
	PeriodMonth     Period = "month"
	PeriodLastMonth Period = "lastMonth"
	PeriodYear      Period = "year"
	PeriodLastYear  Period = "lastYear"
)

var AllPeriods = []Period{
	PeriodToday,
	PeriodYesterday,
	PeriodWeek,
	PeriodLastWeek,
	PeriodMonth,
	PeriodLastMonth,
	PeriodYear,
	PeriodLastYear,
}

func GetPeriodDateRange(period Period) (start, end time.Time) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	switch period {
	case PeriodToday:
		start = today
		end = today.Add(24 * time.Hour)

	case PeriodYesterday:
		start = today.Add(-24 * time.Hour)
		end = today

	case PeriodWeek:
		// Start of this week (Monday)
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = today.Add(-time.Duration(weekday-1) * 24 * time.Hour)
		end = today.Add(24 * time.Hour)

	case PeriodLastWeek:
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		thisWeekStart := today.Add(-time.Duration(weekday-1) * 24 * time.Hour)
		start = thisWeekStart.Add(-7 * 24 * time.Hour)
		end = thisWeekStart

	case PeriodMonth:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		end = today.Add(24 * time.Hour)

	case PeriodLastMonth:
		firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		start = firstOfThisMonth.AddDate(0, -1, 0)
		end = firstOfThisMonth

	case PeriodYear:
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		end = today.Add(24 * time.Hour)

	case PeriodLastYear:
		start = time.Date(now.Year()-1, 1, 1, 0, 0, 0, 0, time.Local)
		end = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
	}

	return start, end
}

type ReportOptions struct {
	Period    Period
	ProjectID *int64
	TagIDs    []int64
}

func GenerateReport(opts ReportOptions) (*model.ReportSummary, error) {
	start, end := GetPeriodDateRange(opts.Period)

	listOpts := db.ListEntriesOptions{
		From:      &start,
		To:        &end,
		ProjectID: opts.ProjectID,
		TagIDs:    opts.TagIDs,
	}

	entries, err := db.ListEntries(listOpts)
	if err != nil {
		return nil, err
	}

	summary := &model.ReportSummary{
		Period:    string(opts.Period),
		StartDate: start,
		EndDate:   end,
		ByProject: make(map[string]time.Duration),
		ByTag:     make(map[string]time.Duration),
		Entries:   make([]model.ReportEntry, 0, len(entries)),
	}

	for _, e := range entries {
		duration := e.Duration()
		summary.TotalDuration += duration

		// Aggregate by project
		if e.Project != nil {
			summary.ByProject[e.Project.Name] += duration
		}

		// Aggregate by tag
		for _, t := range e.Tags {
			summary.ByTag[t.Name] += duration
		}

		// Build tag names
		tagNames := make([]string, len(e.Tags))
		for i, t := range e.Tags {
			tagNames[i] = t.Name
		}

		projectName := ""
		if e.Project != nil {
			projectName = e.Project.Name
		}

		summary.Entries = append(summary.Entries, model.ReportEntry{
			Entry:       e,
			ProjectName: projectName,
			TagNames:    tagNames,
			Duration:    duration,
		})
	}

	return summary, nil
}
