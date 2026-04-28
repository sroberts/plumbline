package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/sroberts/plumbline/internal/buildinfo"
)

func newVersionCmd(stdout io.Writer) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print build version, commit, and signal-set version",
		Long: `Prints plumbline's build version and the signal-set version it ships.

The signal-set version is what CI gates pin via 'assess --signal-set vN'.
See 'plumbline help compatibility' for the policy.

Examples:
  plumbline version
  plumbline version --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := struct {
				Version          string `json:"version"`
				Commit           string `json:"commit"`
				SignalSetVersion string `json:"signal_set_version"`
				Schema           string `json:"schema"`
			}{
				Version:          buildinfo.Version,
				Commit:           buildinfo.Commit,
				SignalSetVersion: buildinfo.SignalSetVersion,
				Schema:           buildinfo.Schema,
			}
			if asJSON {
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			fmt.Fprintf(stdout, "plumbline %s (%s)\nsignal-set: %s\nschema: %s\n",
				info.Version, info.Commit, info.SignalSetVersion, info.Schema)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Emit JSON to stdout.")
	return cmd
}
