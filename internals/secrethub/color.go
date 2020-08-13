package secrethub

import (
	"github.com/spf13/cobra"
	"strconv"

	"github.com/fatih/color"
)

// noColorFlag configures the global behaviour to disable colored output.
type noColorFlag bool

func (f noColorFlag) Type() string {
	return "noColorFlag"
}

// init disables colored output based on the value of the flag.
func (f noColorFlag) init() {
	color.NoColor = bool(f)
}

// RegisterColorFlag registers a color flag that configures whether colored output is used.
func RegisterColorFlag(r *cobra.Command) {
	flag := noColorFlag(false)
	r.PersistentFlags().Var(&flag, "no-color", "Disable colored output.")
}

// String implements the flag.Value interface.
func (f noColorFlag) String() string {
	return strconv.FormatBool(bool(f))
}

// Set disables colors when the given value is false.
func (f *noColorFlag) Set(value string) error {
	b, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	*f = noColorFlag(b)
	f.init()
	return nil
}

// IsBoolFlag makes the flag a boolean flag when used in a Kingpin application.
// Thus, the flag can be used without argument (--color or --no-color).
func (f noColorFlag) IsBoolFlag() bool {
	return true
}
