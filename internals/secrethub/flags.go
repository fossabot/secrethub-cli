package secrethub

import "github.com/spf13/cobra"

func registerTimestampFlag(c *cobra.Command, el *bool) {
	c.Flags().BoolVarP(el, "timestamp", "T", false, "Show timestamps formatted to RFC3339 instead of human readable durations.")
}

func registerForceFlag(c *cobra.Command, el *bool) {
	c.Flags().BoolVarP(el, "force", "f", false, "Ignore confirmation and fail instead of prompt for missing arguments.")
}
