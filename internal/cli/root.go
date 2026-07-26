package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/rtcoder/ytrack/internal/config"
	"github.com/rtcoder/ytrack/internal/youtrack"
	"github.com/spf13/cobra"
)

type Options struct {
	Paths config.Paths
	In    io.Reader
	Out   io.Writer
	Err   io.Writer
}

type app struct {
	paths      config.Paths
	in         io.Reader
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
	in := opts.In
	if in == nil {
		in = os.Stdin
	}

	paths := opts.Paths
	if paths.Global == "" || paths.Local == "" {
		defaultPaths, err := config.DefaultPaths("")
		if err == nil {
			paths = defaultPaths
		}
	}

	a := &app{paths: paths, in: in, out: out, err: errOut}
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
	root.AddCommand(a.newUserCommand())
	root.AddCommand(a.newProjectCommand())

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
		Use:   "show <issue-id>",
		Short: "Show a YouTrack issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadEffective(a.paths)
			if err != nil {
				return err
			}
			if err := config.Require(cfg, "url", "token"); err != nil {
				return err
			}
			issue, raw, err := youtrack.NewClient(cfg.URL, cfg.Token).GetIssue(context.Background(), args[0])
			if err != nil {
				return err
			}
			if a.jsonOutput {
				fmt.Fprintf(a.out, "%s\n", raw)
				return nil
			}
			a.printIssueDetails(issue, cfg.URL)
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

func (a *app) newUserCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Inspect YouTrack users",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "me",
		Short: "Show the current YouTrack user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.newYouTrackClient("url", "token")
			if err != nil {
				return err
			}
			user, raw, err := client.GetMe(context.Background())
			if err != nil {
				return err
			}
			if a.jsonOutput {
				fmt.Fprintf(a.out, "%s\n", raw)
				return nil
			}
			a.printUser(user)
			return nil
		},
	})

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List YouTrack users",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			top, err := cmd.Flags().GetInt("top")
			if err != nil {
				return err
			}
			skip, err := cmd.Flags().GetInt("skip")
			if err != nil {
				return err
			}
			client, err := a.newYouTrackClient("url", "token")
			if err != nil {
				return err
			}
			users, raw, err := client.ListUsers(context.Background(), top, skip)
			if err != nil {
				return err
			}
			if a.jsonOutput {
				fmt.Fprintf(a.out, "%s\n", raw)
				return nil
			}
			a.printUsers(users)
			return nil
		},
	}
	listCmd.Flags().Int("top", 42, "maximum number of users to return")
	listCmd.Flags().Int("skip", 0, "number of users to skip")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "find <query>",
		Short: "Find a single YouTrack user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := a.newYouTrackClient("url", "token")
			if err != nil {
				return err
			}
			user, err := client.ResolveUser(context.Background(), args[0])
			if err != nil {
				return err
			}
			if a.jsonOutput {
				content, err := json.Marshal(user)
				if err != nil {
					return fmt.Errorf("encode user: %w", err)
				}
				fmt.Fprintf(a.out, "%s\n", content)
				return nil
			}
			a.printUser(user)
			return nil
		},
	})
	return cmd
}

func (a *app) newProjectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage YouTrack projects",
	}

	createCmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a YouTrack project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := ""
			if len(args) == 1 {
				title = args[0]
			}
			key, err := cmd.Flags().GetString("key")
			if err != nil {
				return err
			}
			leaderRef, err := cmd.Flags().GetString("leader")
			if err != nil {
				return err
			}
			template, err := cmd.Flags().GetString("template")
			if err != nil {
				return err
			}
			setProjectID, err := cmd.Flags().GetBool("set-project-id")
			if err != nil {
				return err
			}

			interactive := strings.TrimSpace(title) == "" || strings.TrimSpace(key) == "" || strings.TrimSpace(leaderRef) == ""
			reader := bufio.NewReader(a.in)
			if strings.TrimSpace(title) == "" {
				title, err = a.prompt(reader, "Set project name:")
				if err != nil {
					return err
				}
			}
			if strings.TrimSpace(key) == "" {
				key, err = a.prompt(reader, "Set project key:")
				if err != nil {
					return err
				}
			}
			if strings.TrimSpace(leaderRef) == "" {
				leaderRef, err = a.prompt(reader, "Set leader:")
				if err != nil {
					return err
				}
			}
			title = strings.TrimSpace(title)
			key = strings.TrimSpace(key)
			leaderRef = strings.TrimSpace(leaderRef)
			if title == "" {
				return fmt.Errorf("missing project name")
			}
			if key == "" {
				return fmt.Errorf("missing project key")
			}
			if leaderRef == "" {
				return fmt.Errorf("missing project leader")
			}

			client, err := a.newYouTrackClient("url", "token")
			if err != nil {
				return err
			}
			leader, err := client.ResolveUser(context.Background(), leaderRef)
			if err != nil {
				return err
			}
			project, raw, err := client.CreateProject(context.Background(), title, key, leader.ID, template)
			if err != nil {
				return err
			}
			saveProjectID, err := a.shouldSaveProjectID(reader, project.ID, setProjectID, interactive)
			if err != nil {
				return err
			}
			if saveProjectID {
				if err := a.saveLocalProjectID(project.ID); err != nil {
					return err
				}
			}
			if a.jsonOutput {
				fmt.Fprintf(a.out, "%s\n", raw)
				return nil
			}
			fmt.Fprintf(a.out, "Created project %s: %q\n", project.ShortName, project.Name)
			fmt.Fprintf(a.out, "id: %s\n", project.ID)
			if saveProjectID {
				fmt.Fprintf(a.out, "Saved local project_id: %s\n", project.ID)
			}
			return nil
		},
	}
	createCmd.Flags().String("key", "", "project key")
	createCmd.Flags().String("leader", "", "project leader as me, user ID, login, name, or email")
	createCmd.Flags().String("template", "", "project template: scrum or kanban")
	createCmd.Flags().Bool("set-project-id", false, "save the created project ID as the local project_id")
	cmd.AddCommand(createCmd)
	cmd.AddCommand(a.newProjectListCommand())

	return cmd
}

func (a *app) newProjectListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list issues|statuses|users|types|priorities|versions",
		Short: "List project issues and project metadata",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadEffective(a.paths)
			if err != nil {
				return err
			}
			if err := config.Require(cfg, "url", "token", "project_id"); err != nil {
				return err
			}
			client := youtrack.NewClient(cfg.URL, cfg.Token)
			switch args[0] {
			case "issues":
				filters, err := issueFiltersFromFlags(cmd)
				if err != nil {
					return err
				}
				issues, raw, err := client.ListProjectIssuesFiltered(context.Background(), cfg.ProjectID, filters)
				if err != nil {
					return err
				}
				if a.jsonOutput {
					fmt.Fprintf(a.out, "%s\n", raw)
					return nil
				}
				a.printProjectIssues(issues)
			case "users":
				users, raw, err := client.ListProjectUsers(context.Background(), cfg.ProjectID)
				if err != nil {
					return err
				}
				if a.jsonOutput {
					fmt.Fprintf(a.out, "%s\n", raw)
					return nil
				}
				a.printUsers(users)
			case "statuses":
				values, raw, err := client.ListProjectStatuses(context.Background(), cfg.ProjectID)
				if err != nil {
					return err
				}
				if a.jsonOutput {
					fmt.Fprintf(a.out, "%s\n", raw)
					return nil
				}
				a.printNamedValues(values)
			case "types":
				values, raw, err := client.ListProjectTypes(context.Background(), cfg.ProjectID)
				if err != nil {
					return err
				}
				if a.jsonOutput {
					fmt.Fprintf(a.out, "%s\n", raw)
					return nil
				}
				a.printNamedValues(values)
			case "priorities":
				values, raw, err := client.ListProjectPriorities(context.Background(), cfg.ProjectID)
				if err != nil {
					return err
				}
				if a.jsonOutput {
					fmt.Fprintf(a.out, "%s\n", raw)
					return nil
				}
				a.printNamedValues(values)
			case "versions":
				values, raw, err := client.ListProjectVersions(context.Background(), cfg.ProjectID)
				if err != nil {
					return err
				}
				if a.jsonOutput {
					fmt.Fprintf(a.out, "%s\n", raw)
					return nil
				}
				a.printNamedValues(values)
			default:
				return fmt.Errorf("unknown project list resource %q", args[0])
			}
			return nil
		},
	}
	cmd.Flags().String("status", "", "filter issues by status")
	cmd.Flags().String("user", "", "filter issues by assignee")
	cmd.Flags().String("type", "", "filter issues by type")
	cmd.Flags().String("priority", "", "filter issues by priority")
	return cmd
}

func (a *app) newYouTrackClient(fields ...string) (*youtrack.Client, error) {
	cfg, err := config.LoadEffective(a.paths)
	if err != nil {
		return nil, err
	}
	if err := config.Require(cfg, fields...); err != nil {
		return nil, err
	}
	return youtrack.NewClient(cfg.URL, cfg.Token), nil
}

func (a *app) prompt(reader *bufio.Reader, label string) (string, error) {
	fmt.Fprintln(a.out, label)
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func (a *app) shouldSaveProjectID(reader *bufio.Reader, projectID string, setProjectID, interactive bool) (bool, error) {
	if setProjectID {
		return true, nil
	}
	if !interactive {
		return false, nil
	}

	cfg, err := config.Load(a.paths.Local)
	if err != nil {
		return false, err
	}
	if cfg.ProjectID != "" {
		return a.confirm(reader, fmt.Sprintf("Local project_id is %s. Overwrite with %s? [y/N]", cfg.ProjectID, projectID))
	}
	return a.confirm(reader, "Set new project as local project_id? [y/N]")
}

func (a *app) confirm(reader *bufio.Reader, label string) (bool, error) {
	answer, err := a.prompt(reader, label)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes"), nil
}

func (a *app) saveLocalProjectID(projectID string) error {
	cfg, err := config.Load(a.paths.Local)
	if err != nil {
		return err
	}
	cfg.ProjectID = projectID
	return config.Save(a.paths.Local, cfg)
}

func (a *app) printConfig(cfg config.Config) {
	fmt.Fprintf(a.out, "url: %s\n", cfg.URL)
	fmt.Fprintf(a.out, "token: %s\n", config.MaskToken(cfg.Token))
	fmt.Fprintf(a.out, "project_id: %s\n", cfg.ProjectID)
}

func (a *app) printUser(user youtrack.User) {
	fmt.Fprintf(a.out, "id: %s\n", user.ID)
	fmt.Fprintf(a.out, "login: %s\n", user.Login)
	fmt.Fprintf(a.out, "name: %s\n", userDisplayName(user))
	if user.Email != "" {
		fmt.Fprintf(a.out, "email: %s\n", user.Email)
	}
	if user.Banned {
		fmt.Fprintln(a.out, "status: banned")
	}
}

func (a *app) printUsers(users []youtrack.User) {
	w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	for _, user := range users {
		status := ""
		if user.Banned {
			status = "banned"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", user.ID, user.Login, userDisplayName(user), status)
	}
	_ = w.Flush()
}

func (a *app) printProjectIssues(issues []youtrack.ProjectIssue) {
	w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	for _, issue := range issues {
		fmt.Fprintf(w, "%s\t%s\t%s\n", issue.IDReadable, issueFieldValue(issue, "State"), issue.Summary)
	}
	_ = w.Flush()
}

func (a *app) printIssueDetails(issue youtrack.ProjectIssue, baseURL string) {
	fmt.Fprintf(a.out, "id: %s\n", issue.IDReadable)
	fmt.Fprintf(a.out, "title: %s\n", issue.Summary)
	if issue.Description != "" {
		fmt.Fprintf(a.out, "description: %s\n", issue.Description)
	}
	fmt.Fprintf(a.out, "state: %s\n", issueFieldValue(issue, "State"))
	fmt.Fprintf(a.out, "assignee: %s\n", issueFieldValue(issue, "Assignee"))
	fmt.Fprintf(a.out, "priority: %s\n", issueFieldValue(issue, "Priority"))
	fmt.Fprintf(a.out, "url: %s/issue/%s\n", strings.TrimRight(baseURL, "/"), issue.IDReadable)
}

func (a *app) printNamedValues(values []youtrack.NamedValue) {
	w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	for _, value := range values {
		fmt.Fprintf(w, "%s\t%s\n", value.ID, value.Name)
	}
	_ = w.Flush()
}

func userDisplayName(user youtrack.User) string {
	if user.FullName != "" {
		return user.FullName
	}
	return user.Name
}

func issueFieldValue(issue youtrack.ProjectIssue, name string) string {
	for _, field := range issue.CustomFields {
		if field.Name != name {
			continue
		}
		switch value := field.Value.(type) {
		case map[string]any:
			if login, ok := value["login"].(string); ok {
				return login
			}
			if name, ok := value["name"].(string); ok {
				return name
			}
		case []any:
			var parts []string
			for _, item := range value {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if name, ok := itemMap["name"].(string); ok {
					parts = append(parts, name)
				}
			}
			return strings.Join(parts, ",")
		case string:
			return value
		}
	}
	return ""
}

func issueFiltersFromFlags(cmd *cobra.Command) (youtrack.IssueFilters, error) {
	status, err := cmd.Flags().GetString("status")
	if err != nil {
		return youtrack.IssueFilters{}, err
	}
	user, err := cmd.Flags().GetString("user")
	if err != nil {
		return youtrack.IssueFilters{}, err
	}
	issueType, err := cmd.Flags().GetString("type")
	if err != nil {
		return youtrack.IssueFilters{}, err
	}
	priority, err := cmd.Flags().GetString("priority")
	if err != nil {
		return youtrack.IssueFilters{}, err
	}
	return youtrack.IssueFilters{
		Status:   status,
		User:     user,
		Type:     issueType,
		Priority: priority,
	}, nil
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
