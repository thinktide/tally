package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/thinktide/tally/internal/config"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
	"github.com/thinktide/tally/internal/service"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var reportFormat string

var reportCmd = &cobra.Command{
	Use:   "report [period] [@project] [+tag]...",
	Short: "Generate time reports",
	Long: `Generate time reports for various periods.

Periods:
  today, yesterday, week, lastWeek, month, lastMonth, year, lastYear

Examples:
  tally report                    # Interactive menu
  tally report today              # Today's report
  tally report week @work         # This week's report for 'work' project
  tally report month +backend     # This month's report with 'backend' tag
  tally report --format json      # Output as JSON`,
	RunE: runReport,
}

func init() {
	reportCmd.Flags().StringVar(&reportFormat, "format", "", "Output format: table, json, csv")
}

func runReport(cmd *cobra.Command, args []string) error {
	// Get default format from config
	if reportFormat == "" {
		format, err := config.Get(config.KeyOutputFormat)
		if err != nil {
			return err
		}
		reportFormat = format
	}

	opts := service.ReportOptions{}

	// Parse arguments
	for _, arg := range args {
		if strings.HasPrefix(arg, "@") {
			projectName := strings.TrimPrefix(arg, "@")
			project, err := db.GetProjectByName(projectName)
			if err != nil {
				return fmt.Errorf("failed to get project: %w", err)
			}
			if project == nil {
				fmt.Printf("Project @%s not found\n", projectName)
				return nil
			}
			opts.ProjectID = &project.ID
		} else if strings.HasPrefix(arg, "+") {
			tagName := strings.TrimPrefix(arg, "+")
			tag, err := db.GetTagByName(tagName)
			if err != nil {
				return fmt.Errorf("failed to get tag: %w", err)
			}
			if tag == nil {
				fmt.Printf("Tag +%s not found\n", tagName)
				return nil
			}
			opts.TagIDs = append(opts.TagIDs, tag.ID)
		} else {
			// Assume it's a period
			opts.Period = service.Period(arg)
		}
	}

	// If no period specified, show interactive menu
	if opts.Period == "" {
		period, err := selectPeriod()
		if err != nil {
			return err
		}
		opts.Period = period
	}

	// Validate period
	validPeriod := false
	for _, p := range service.AllPeriods {
		if opts.Period == p {
			validPeriod = true
			break
		}
	}
	if !validPeriod {
		return fmt.Errorf("invalid period: %s\nValid periods: %v", opts.Period, service.AllPeriods)
	}

	// Generate report
	summary, err := service.GenerateReport(opts)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Output report
	switch reportFormat {
	case "json":
		return outputJSON(summary)
	case "csv":
		return outputCSV(summary)
	default:
		return outputTable(summary)
	}
}

func selectPeriod() (service.Period, error) {
	fmt.Println("Select a report period:")
	fmt.Println()
	for i, p := range service.AllPeriods {
		fmt.Printf("  %d. %s\n", i+1, p)
	}
	fmt.Println()
	fmt.Print("Enter number (1-8): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	var choice int
	if _, err := fmt.Sscanf(input, "%d", &choice); err != nil {
		return "", fmt.Errorf("invalid selection")
	}

	if choice < 1 || choice > len(service.AllPeriods) {
		return "", fmt.Errorf("invalid selection: choose 1-%d", len(service.AllPeriods))
	}

	return service.AllPeriods[choice-1], nil
}

func outputTable(summary *model.ReportSummary) error {
	fmt.Printf("\nReport: %s\n", summary.Period)
	fmt.Printf("Period: %s to %s\n",
		summary.StartDate.Format("2006-01-02"),
		summary.EndDate.Add(-1).Format("2006-01-02"))
	fmt.Printf("Total: %s\n\n", formatDuration(summary.TotalDuration))

	if len(summary.ByProject) > 0 {
		fmt.Println("By Project:")
		table := tablewriter.NewWriter(os.Stdout)
		table.SetBorder(false)
		table.SetHeaderLine(false)
		table.SetColumnSeparator("")
		table.SetTablePadding("  ")

		for name, dur := range summary.ByProject {
			table.Append([]string{"  @" + name, formatDurationShort(dur)})
		}
		table.Render()
		fmt.Println()
	}

	if len(summary.ByTag) > 0 {
		fmt.Println("By Tag:")
		table := tablewriter.NewWriter(os.Stdout)
		table.SetBorder(false)
		table.SetHeaderLine(false)
		table.SetColumnSeparator("")
		table.SetTablePadding("  ")

		for name, dur := range summary.ByTag {
			table.Append([]string{"  +" + name, formatDurationShort(dur)})
		}
		table.Render()
		fmt.Println()
	}

	if len(summary.Entries) > 0 {
		fmt.Println("Entries:")
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader([]string{"ID", "Project", "Title", "Duration", "Tags", "Date"})
		table.SetBorder(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("")
		table.SetColumnSeparator("")
		table.SetRowSeparator("")
		table.SetHeaderLine(false)
		table.SetTablePadding("  ")
		table.SetNoWhiteSpace(true)
		table.SetAutoWrapText(false)

		for _, e := range summary.Entries {
			title := e.Title
			if len(title) > 35 {
				title = title[:32] + "..."
			}
			table.Append([]string{
				e.ID,
				"@" + e.ProjectName,
				title,
				formatDurationShort(e.Duration),
				strings.Join(e.TagNames, ", "),
				e.StartTime.Format("2006-01-02 15:04"),
			})
		}
		table.Render()
	}

	return nil
}

func outputJSON(summary *model.ReportSummary) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

func outputCSV(summary *model.ReportSummary) error {
	writer := csv.NewWriter(os.Stdout)
	defer writer.Flush()

	// Header
	writer.Write([]string{"ID", "Project", "Title", "Duration (minutes)", "Tags", "Start", "End"})

	for _, e := range summary.Entries {
		endTime := ""
		if e.EndTime != nil {
			endTime = e.EndTime.Format("2006-01-02 15:04:05")
		}

		writer.Write([]string{
			e.ID,
			e.ProjectName,
			e.Title,
			fmt.Sprintf("%.1f", e.Duration.Minutes()),
			strings.Join(e.TagNames, ","),
			e.StartTime.Format("2006-01-02 15:04:05"),
			endTime,
		})
	}

	return nil
}
