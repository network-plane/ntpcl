package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

func TestPersistentSetFlag(t *testing.T) {
	var opts commandOptions
	root := &cobra.Command{Use: "ntpcl"}
	addCommonFlags(root, &opts)

	ntpCmd := &cobra.Command{
		Use:  "ntp",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	root.AddCommand(ntpCmd)

	root.SetArgs([]string{"--set", "ntp", "127.0.0.1"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !opts.set {
		t.Fatal("expected --set on root to apply to subcommand via persistent flags")
	}
}

func TestRequireSingleArgShowsHelp(t *testing.T) {
	cmd := &cobra.Command{Use: "ntp SERVER"}
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := requireSingleArg("ntp", "an NTP server address")(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("missing")) {
		t.Fatalf("expected missing arg message, got %v", err)
	}
}

func TestExitCodeForNetwork(t *testing.T) {
	if got := exitCodeFor(os.ErrDeadlineExceeded); got == exitOK {
		t.Fatalf("expected non-zero exit code")
	}
}
