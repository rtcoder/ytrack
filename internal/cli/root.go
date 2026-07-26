package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rtcoder/ytrack/internal/config"
	"github.com/rtcoder/ytrack/internal/youtrack"
	"github.com/spf13/cobra"
)

type Options struct {
	Paths config.Paths
	Out   io.Writer
	Err   io.Writer
}

type app struct {
	paths      config.Paths
	out        io.Writer
	err        io.Writer
	jsonOutput bool
}

func NewRootCommand(opts Options) *cobra.Command {
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	errOut := opts.Err
	if errOut == nil {
		errOut = os.Stderr
	}

	paths := opts.Paths
	if paths.Global == "" || paths.Local == "" {
		defaultPaths, err := config.DefaultPaths("")
		if err == nil {
			paths = defaultPaths
		}
	}

	a := &app{paths: paths, out: out, err: errOut}
	root := &cobra.Command{
		Use:           "ytrack",
		Short:         "Manage YouTrack issues from your terminal",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().BoolVar(&a.jsonOutput, "json", false, "print raw JSON responses")

	root.AddCommand(a.newGlobalCommand())
	root.AddCommand(a.newLocalConfigCommand("set-url", "Set local YouTrack URL", "url", "url"))
	root.AddCommand(a.newLocalConfigCommand("set-token", "Set local YouTrack token", "token", "token"))
	root.AddCommand(a.newLocalConfigCommand("set-project-id", "Set local YouTrack project ID", "project-id", "project_id"))
	root.AddCommand(a.newLocalUnsetCommand("unset-url", "Unset local YouTrack URL", "url"))
	root.AddCommand(a.newLocalUnsetCommand("unset-token", "Unset local YouTrack token", "token"))
	root.AddCommand(a.newLocalUnsetCommand("unset-project-id", "Unset local YouTrack project ID", "project_id"))
	root.AddCommand(a.newShowCommand())
	root.AddCommand(a.newIssueCommand())

	return root
}

func (a *app) newGlobalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "global",
		Short: "Manage global ytrack configuration",
	}
	cmd.AddCommand(a.newGlobalSetCommand("set-url", "Set global YouTrack URL", "url", "url"))
	cmd.AddCommand(a.newGlobalSetCommand("set-token", "Set global YouTrack token", "token", "token"))
	cmd.AddCommand(a.newGlobalUnsetCommand("unset-url", "Unset global YouTrack URL", "url"))
	cmd.AddCommand(a.newGlobalUnsetCommand("unset-token", "Unset global YouTrack token", "token"))
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show global configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(a.paths.Global)
			if err != nil {
				return err
			}
			a.printConfig(cfg)
			return nil
		},
	})
	return cmd
}

func (a *app) newGlobalSetCommand(use, short, argName, field string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <" + argName + ">",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(a.paths.Global)
			if err != nil {
				return err
			}
			setField(&cfg, field, args[0])
			cfg.ProjectID = ""
			if err := config.Save(a.paths.Global, cfg); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Saved global %s\n", field)
			return nil
		},
	}
}

func (a *app) newGlobalUnsetCommand(use, short, field string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(a.paths.Global)
			if err != nil {
				return err
			}
			setField(&cfg, field, "")
			cfg.ProjectID = ""
			if err := config.Save(a.paths.Global, cfg); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Unset global %s\n", field)
			return nil
		},
	}
}

func (a *app) newLocalConfigCommand(use, short, argName, field string) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <" + argName + ">",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(a.paths.Local)
			if err != nil {
				return err
			}
			setField(&cfg, field, args[0])
			if err := config.Save(a.paths.Local, cfg); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Saved local %s\n", field)
			return nil
		},
	}
}

func (a *app) newLocalUnsetCommand(use, short, field string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(a.paths.Local)
			if err != nil {
				return err
			}
			setField(&cfg, field, "")
			if err := config.Save(a.paths.Local, cfg); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "Unset local %s\n", field)
			return nil
		},
	}
}

func (a *app) newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show effective configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadEffective(a.paths)
			if err != nil {
				return err
			}
			a.printConfig(cfg)
			return nil
		},
	}
}

func (a *app) newIssueCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage YouTrack issues",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "create <summary> [description]",
		Short: "Create a YouTrack issue",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadEffective(a.paths)
			if err != nil {
				return err
			}
			if err := config.Require(cfg, "url", "token", "project_id"); err != nil {
				return err
			}
			description := ""
			if len(args) == 2 {
				description = args[1]
			}
			issue, raw, err := youtrack.NewClient(cfg.URL, cfg.Token).CreateIssue(context.Background(), cfg.ProjectID, args[0], description)
			if err != nil {
				return err
			}
			if a.jsonOutput {
				fmt.Fprintf(a.out, "%s\n", raw)
				return nil
			}
			fmt.Fprintf(a.out, "Created %s: %q\n", issue.IDReadable, issue.Summary)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status <issue-id> <status>",
		Short: "Set a YouTrack issue status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadEffective(a.paths)
			if err != nil {
				return err
			}
			if err := config.Require(cfg, "url", "token"); err != nil {
				return err
			}
			result, raw, err := youtrack.NewClient(cfg.URL, cfg.Token).SetStatus(context.Background(), args[0], args[1])
			if err != nil {
				return err
			}
			if a.jsonOutput {
				fmt.Fprintf(a.out, "%s\n", raw)
				return nil
			}
			if len(result.Issues) > 0 {
				fmt.Fprintf(a.out, "Updated %s to %s\n", result.Issues[0].IDReadable, args[1])
				return nil
			}
			fmt.Fprintf(a.out, "Updated %s to %s\n", args[0], args[1])
			return nil
		},
	})
	return cmd
}

func (a *app) printConfig(cfg config.Config) {
	fmt.Fprintf(a.out, "url: %s\n", cfg.URL)
	fmt.Fprintf(a.out, "token: %s\n", config.MaskToken(cfg.Token))
	fmt.Fprintf(a.out, "project_id: %s\n", cfg.ProjectID)
}

func setField(cfg *config.Config, field, value string) {
	switch field {
	case "url":
		cfg.URL = value
	case "token":
		cfg.Token = value
	case "project_id":
		cfg.ProjectID = value
	}
}
