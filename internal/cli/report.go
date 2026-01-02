package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/thinktide/tally/internal/config"
	"github.com/thinktide/tally/internal/db"
	"github.com/thinktide/tally/internal/model"
	"github.com/thinktide/tally/internal/service"
)

// reportFormat defines the output format for the report generation.
//
// It can be set to specific formats such as "table", "json", or "csv" to control how the report data is presented.
// If not explicitly set, it may fall back to a default value from the configuration.
var reportFormat string

// reportCmd is a command that generates time reports for specified periods, projects, and tags.
//
// The command supports various time periods such as "today", "week", or "lastMonth".
// Users can filter reports by specifying a project (using "@project") and/or tags (using "+tag").
//
// When executed, reportCmd parses the input arguments and generates a time summary.
// The summary can be output in different formats such as table, JSON, and CSV.
//
// If no period is provided, the command offers an interactive menu to select a time period.
//
// Errors may occur if an invalid period is provided, or if specified projects or tags are not found.
// The generated report includes aggregated durations by project and tags, with detailed entry data.
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

// init configures flags for the [reportCmd] command.
//
// The function binds the "format" flag to the variable reportFormat, allowing the use of different output formats:
//   - format: Accepts "table", "json", or "csv" as values.
//
// This setup enables users to customize the output format when generating reports.
func init() {
	reportCmd.Flags().StringVar(&reportFormat, "format", "", "Output format: table, json, csv")
}

// runReport generates a report based on the provided options and arguments, and outputs it in the desired format.
//
// The function processes command arguments `args` to determine the report's period, project, and tags. If a period
// is not specified, it prompts the user to select one interactively.
//
// - `cmd`: Represents the cobra.Command instance associated with this operation.
// - `args`: Contains the arguments to specify the report scope, including period, project, or tags.
//
// The function validates the period, fetches data, and formats the report based on the configured or default output
// format. Supported formats include JSON, CSV, and tabular rendering.
//
// Returns an error if any validation, data fetching, or report generation step fails.
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

// selectPeriod prompts the user to select a reporting period interactively.
//
// It displays a numbered list of all available periods from [service.AllPeriods], allowing the user to choose one.
// If user input is invalid, an error is returned.
//
// Returns:
//   - The selected [service.Period].
//   - An error if input is invalid or reading input fails.
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

// outputTable generates and renders a formatted table from the given [model.ReportSummary].
//
// It displays data such as report period, individual entries, summaries by project, and summaries by tag in a human-readable table.
// The function uses the [tablewriter] package to format the table output and ensures that long text is truncated for better readability.
//
// summary contains aggregated report details including total duration, grouped data by tags and projects, and individual entries.
// If summary contains no entries or group data, only the total duration is printed.
//
// Returns nil upon successful execution or an error if there is an issue with the output generation.
func outputTable(summary *model.ReportSummary) error {
	fmt.Printf("\nReport: %s\n", summary.Period)
	fmt.Printf("Period: %s to %s\n\n",
		summary.StartDate.Format("2006-01-02"),
		summary.EndDate.Add(-1).Format("2006-01-02"))

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
		fmt.Println()
	}

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

	fmt.Printf("Total: %s\n", formatDuration(summary.TotalDuration))

	return nil
}

// outputJSON writes the provided [model.ReportSummary] to the standard output in JSON format with indentation.
//
// The function uses a JSON encoder to serialize the [model.ReportSummary] object and ensures the output is formatted
// in a human-readable manner with proper indentation.
//
// summary is the [model.ReportSummary] to be serialized and output.
//
// Returns an error if the encoding process fails, which might indicate issues such as the inability to write to stdout.
func outputJSON(summary *model.ReportSummary) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

// outputCSV writes the provided [model.ReportSummary] in CSV format to the standard output.
//
// The CSV output includes a header row followed by one row per entry in the report summary. Each row contains:
// - the entry ID,
// - project name,
// - title,
// - duration in minutes,
// - associated tags,
// - start time, and
// - end time (if available).
//
// If the report summary contains no entries, only the header row will be written.
//
// Returns an error if writing to the CSV writer fails. Use [csv.NewWriter] for output formatting consistency.
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
