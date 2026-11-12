// Copyright 2026 Printing Press Library contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// version is the CLI version printed by --version.
var version = "0.1.0"

type rootFlags struct {
	asJSON       bool
	timeout      time.Duration
	rateLimit    float64
	dryRun       bool
	agent        bool
	selectFields string
	apiBase      string
	pageBase     string
}

// Execute runs the CLI.
func Execute() error {
	return ExecuteArgs(os.Args[1:], os.Stdout, os.Stderr)
}

// ExecuteArgs runs the CLI with explicit args and writers. Tests use it.
func ExecuteArgs(args []string, stdout, stderr io.Writer) error {
	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func newRootCmd() *cobra.Command {
	flags := &rootFlags{}
	rootCmd := &cobra.Command{
		Use:   "your-golf-booking-pp-cli",
		Short: "Read public Your Golf Booking occupancy for one venue at a time",
		Long: `Read signed-out public occupancy from Your Golf Booking, the calendar behind TrackMan Booking pages such as booking.trackmangolf.com.

Pass a venue slug or a public booking URL (/booking, /booking/bays, or /booking/simulators). There is no public directory of every venue, so this CLI does not search the domain for clubs.

The public bookings API returns past dates, one day at a time. Summary fill rates are patterns in that public calendar. They are not sales history.

No login, checkout, cart, or account is used.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	rootCmd.SetVersionTemplate("your-golf-booking-pp-cli {{ .Version }}\n")

	rootCmd.PersistentFlags().BoolVar(&flags.asJSON, "json", false, "Output JSON")
	rootCmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Request timeout")
	rootCmd.PersistentFlags().Float64Var(&flags.rateLimit, "rate-limit", 1, "Maximum public requests per second (0 disables the pause)")
	rootCmd.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "Show the request plan and send nothing")
	rootCmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Shortcut for --json")
	rootCmd.PersistentFlags().StringVar(&flags.selectFields, "select", "", "Comma-separated JSON fields to keep on the top level and on each venue")
	rootCmd.PersistentFlags().StringVar(&flags.apiBase, "api-base", "https://api.yourgolfbooking.com", "Public API origin")
	rootCmd.PersistentFlags().StringVar(&flags.pageBase, "page-base", "https://booking.trackmangolf.com", "Public booking page origin")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if flags.agent && !cmd.Flags().Changed("json") {
			flags.asJSON = true
		}
		if flags.rateLimit < 0 {
			return fmt.Errorf("--rate-limit must be 0 or greater")
		}
		if flags.timeout < 0 {
			return fmt.Errorf("--timeout must be 0 or greater")
		}
		return nil
	}

	rootCmd.AddCommand(newVenueCmd(flags))
	rootCmd.AddCommand(newOccupancyCmd(flags))
	rootCmd.AddCommand(newSummaryCmd(flags))
	rootCmd.AddCommand(newDoctorCmd(flags))
	return rootCmd
}
