package main

import (
	"fmt"
	"os"
	"os/exec"
	p "path"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/msteffen/golang-time-tracker/client"
	watchd "github.com/msteffen/golang-time-tracker/watch_daemon"
)

const (
	s_Minute = 60
	s_Hour   = 60 * s_Minute
)

var (
	/* const */ dataDir = os.Getenv("HOME") + "/.time-tracker"
	/* const */ dbFile = dataDir + "/db"
	/* const */ logFile = dataDir + "/watchd.stdout"
	/* const */ errFile = dataDir + "/watchd.stderr"
	/* const */ socketFile = dataDir + "/sock"

	binaryName string // populated by main(), used by by getCLIClient()
)

type couldNotConnectErr struct {
	innerErr error
}

func (e couldNotConnectErr) Error() string {
	return "could not connect to time-tracker watch daemon (even after attempting to start it): " + e.innerErr.Error()
}

func getCLIClient() (*client.Client, error) {
	var err error
	for retry := 0; retry < 2; retry++ {
		// Try to connect naively
		c := client.GetClient(socketFile)
		_, err = c.Status()
		if err == nil {
			return c, nil
		} else if retry > 0 {
			break
		}

		// Try to connect to watchd, or start it if it's not running
		fmt.Printf("could not connect to server: %v\nAttempting to start it...\n", err)
		cmd := exec.Command(binaryName, "serve") // run "t serve" in another process
		cmd.Stdout, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("could not create stdout log for watchd: %v", err)
		}
		cmd.Stderr, err = os.OpenFile(errFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("could not create stderr log for watchd: %v", err)
		}
		cmd.Stdout.Write([]byte("\n--------------------------------------------\n"))
		cmd.Stderr.Write([]byte("\n--------------------------------------------\n"))
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("error starting watchd: %v", err)
		}
		time.Sleep(time.Second) // wait for watchd to start
	}
	return nil, couldNotConnectErr{err}
}

// morning returns the earliest time that is in the same day as 'morning'
func morning(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

// dayRow reads the intervals for the day starting at 'morning', renders those
// intervals, and returns a bar for the day
func dayRow(c *client.Client, morning time.Time, includeEndGap bool) (string, error) {
	resp, err := c.GetIntervals(morning, morning.Add(24*time.Hour))
	if err != nil {
		return "", fmt.Errorf("could not retrieve today's intervals: %v", err)
	}
	workDuration := time.Duration(0)
	for idx, i := range resp.Intervals {
		// don't count intervals that are too short
		if i.End-i.Start < s_Hour {
			continue
		}
		workDuration += time.Duration(i.End-i.Start) * time.Second
		// remove resp.EndGap (but add it below the "too short" check above, so that
		// resp.EndGap is only removed if the in-progress interval was long enough
		// to be added to the total)
		// TODO get rid of EndGap
		if idx+1 == len(resp.Intervals) {
			workDuration -= (time.Duration(resp.EndGap) * time.Second)
		}
	}

	// The API proactively extends the last interval to now. If we want to render
	// today without extension (we blink "unfinished" time) then remove it from
	// the last interval
	// TODO get rid of EndGap!
	// Also, show how much time is remaining until the in-progress interval counts
	// (EndGap > 0 => last interval is in-progress)
	durationStr := fmt.Sprintf(
		"%dh%dm", workDuration/time.Hour, workDuration%time.Hour/time.Minute)
	if resp.EndGap > 0 {
		i := &resp.Intervals[len(resp.Intervals)-1]
		if remaining := s_Hour - (i.End - i.Start) + resp.EndGap; remaining > 0 {
			durationStr += fmt.Sprintf(" (%dm to go)", remaining/s_Minute)
		}
		if !includeEndGap {
			i.End -= resp.EndGap
		}
	}

	// Return string of form "Mon 02/01 [...bar...] 4h20m"
	// Can't use go duration.String() since we don't want seconds
	return fmt.Sprintf("%[1]s%[2]s%[3]s %[4]s %[1]s%[5]s%[3]s",
		sgr(boldText, setFGColor, barColor),
		morning.Format("Mon 01/02:"),
		string(sgr(resetAll)),
		Bar(morning, resp.Intervals),
		durationStr), nil
}

func weekCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "week",
		Short: "Show this week's time-tracking activity",
		Long:  "Show this week's time-tracking activity",
		Run: BoundedCommand(0, 0, func(args []string) error {
			c, err := getCLIClient()
			if err != nil {
				return err
			}

			for tick := 0; ; tick = 2 - ((tick + 1) % 2) {
				// Note: Now() sets local time, which is necessary for watchd (which
				// assumes Local time for all ticks)
				start := morning(time.Now()).Add(-6 * 24 * time.Hour)
				if tick > 0 {
					// tick starts at 0 and then alternates between 1 and 2
					fmt.Printf("\x1b[9F\x1b[J") // up nine lines & clear rest of screen
				}
				for day := 0; day < 7; day++ {
					// print upper border
					if day+1 == 7 {
						fmt.Println(strings.Repeat("-", 80))
					}
					// print bar itself
					bar, err := dayRow(c, start, tick == 1)
					if err != nil {
						return err
					}
					fmt.Println(bar)
					// print lower border
					if day+1 == 7 {
						fmt.Println(strings.Repeat("-", 80))
					}
					// advance to next day
					start = start.Add(24 * time.Hour)
				}
				time.Sleep(time.Second)
			}
			return nil
		}),
	}
}

func watchCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "watch <directory>",
		Short: "Start watching the given project directory for writes",
		Long:  "Start watching the given project directory for writes",
		Run: BoundedCommand(1, 1, func(args []string) error {
			c, err := getCLIClient()
			if err != nil {
				return err
			}

			// Construct dir to send to watchd (must be clean absolute path)
			dir := p.Clean(args[0])
			if !p.IsAbs(dir) {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("watch dir %q is relative, but could not get current working dir: %v", dir, err)
				}
				dir = p.Join(wd, dir)
			}
			return c.Watch(dir, label)
		}),
	}
	cmd.Flags().StringVarP(&label, "label", "l", "", "project label associated with the watched dir")
	return cmd
}

func tickCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tick <label>",
		Short: "Append a tick (work event) with the given label",
		Long:  "Append a tick (work event) with the given label",
		Run: BoundedCommand(1, 1, func(args []string) error {
			c, err := getCLIClient()
			if err != nil {
				return err
			}
			return c.Tick(args[0])
		}),
	}
}

func getWatchedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watched",
		Short: "Print the directories being watched currently by the watch daemon",
		Long:  "Print the directories being watched currently by the watch daemon",
		Run: BoundedCommand(0, 0, func(args []string) error {
			c, err := getCLIClient()
			if err != nil {
				return err
			}
			resp, err := c.GetWatches()
			if err != nil {
				return fmt.Errorf("could not retrieve watches: %v", err)
			}
			for _, wi := range resp.Watches {
				fmt.Printf("%s (%s)\n", wi.Dir, wi.Label)
			}
			return nil
		}),
	}
}

func serveCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the time-tracker watch daemon",
		Long:  "Start the time-tracker watch daemon",
		Run: BoundedCommand(0, 0, func(_ []string) error {
			// typically this is run by systemd, so use compact logs
			if !verbose {
				log.SetLevel(log.WarnLevel)
			}

			// Set up standard serving dir
			if info, err := os.Stat(dataDir); err != nil {
				if err := os.Mkdir(dataDir, 0755); err != nil {
					return err
				}
			} else if info.Mode().Perm()&0700 != 0700 {
				return fmt.Errorf("must have rwx permissions on %s but only have %s (%0d vs 0700)",
					dataDir, info.Mode(), info.Mode().Perm()&0700)
			}
			apiServer, err := watchd.NewServer(watchd.SystemClock, dbFile)
			if err != nil {
				return fmt.Errorf("could not create APIServer: %v", err)
			}
			return watchd.ServeOverHTTP(socketFile, watchd.SystemClock, apiServer)
		}),
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "If set, increase the logging verbosity to include every request/response")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Give the status of the time-tracker watch daemon (or start if it it's stopped)",
		Long:  "Give the status of the time-tracker watch daemon (or start if it it's stopped)",
		Run: BoundedCommand(0, 0, func(args []string) error {
			c, err := getCLIClient()
			if err != nil {
				return err
			}

			// Call /status and print uptime
			// TODO(msteffen): getCLIClient also calls /status, so this is redundant
			resp, err := c.Status()
			if err != nil {
				return fmt.Errorf("error retrieving daemon status: %v", err)
			}
			fmt.Printf("time-tracker has been up for %s", resp)
			return nil
		}),
	}
}

func main() {
	rootCmd := cobra.Command{
		Use:   "t",
		Short: "T is the client for the golang-time-tracker server",
		Long: "Client-side CLI for a time-tracking/time-gamifying tool that helps " +
			"distractable people use their time more mindfully",
		Run: BoundedCommand(0, 0, func(_ []string) error {
			c, err := getCLIClient()
			if err != nil {
				return err
			}

			// Get today's time worked and print a bar
			// Now() sets local time, which is important--see
			// https://github.com/msteffen/golang-time-tracker/issues/2)
			morning := morning(time.Now()) // "start" at the day before today, for the loop
			for tick := 0; ; tick = 2 - ((tick + 1) % 2) {
				if tick > 0 {
					// tick starts at 0 and then alternates between 1 and 2
					fmt.Printf("\x1b[1F\x1b[K") // jump up one line & clear it
				}
				bar, err := dayRow(c, morning, tick == 1)
				if err != nil {
					return err
				}
				fmt.Println(bar)
				time.Sleep(time.Second)
			}
			return nil
		}),
	}
	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(tickCmd())
	rootCmd.AddCommand(getWatchedCmd())
	rootCmd.AddCommand(weekCmd())
	rootCmd.AddCommand(watchCmd())

	binaryName = os.Args[0]
	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
