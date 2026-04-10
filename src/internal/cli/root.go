package cli

import (
	"github.com/spf13/cobra"
)

// CreateRootCommand builds the root cobra command with all subcommands.
func CreateRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "oss-cli",
		Short: "OSS Pages CLI - deploy static sites",
	}

	// init command
	initCmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Initialize a new project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			buildCmd, _ := cmd.Flags().GetString("build-command")
			outputDir, _ := cmd.Flags().GetString("output-dir")
			server, _ := cmd.Flags().GetString("server")
			return RunInit(args[0], buildCmd, outputDir, server)
		},
	}
	initCmd.Flags().String("build-command", "npm run build", "Build command")
	initCmd.Flags().String("output-dir", "dist", "Output directory")
	initCmd.Flags().String("server", "https://api.example.com", "Server URL")
	root.AddCommand(initCmd)

	// deploy command
	var deployServer string
	var deployConfig string
	deployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Build and deploy project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunDeploy(cmd.Context(), deployServer, deployConfig)
		},
	}
	deployCmd.Flags().StringVar(&deployServer, "server", "", "Server URL (overrides wrangler.toml)")
	deployCmd.Flags().StringVarP(&deployConfig, "config", "c", "", "Config file path")
	root.AddCommand(deployCmd)

	// projects command group
	var projectsServer string
	projectsCmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
	}
	projectsCmd.PersistentFlags().StringVar(&projectsServer, "server", "", "Server URL")

	projectsCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListProjects(cmd.Context(), projectsServer)
		},
	})
	projectsCmd.AddCommand(&cobra.Command{
		Use:   "view [name]",
		Short: "View project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ViewProject(cmd.Context(), projectsServer, args[0])
		},
	})
	projectsCmd.AddCommand(&cobra.Command{
		Use:   "delete [name]",
		Short: "Delete project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return DeleteProject(cmd.Context(), projectsServer, args[0])
		},
	})
	root.AddCommand(projectsCmd)

	return root
}

// Execute runs the root command.
func Execute() error {
	return CreateRootCommand().Execute()
}

