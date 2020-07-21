package cmd

import (
	"fmt"
	"log"

	"github.com/rahveiz/topomate/project"

	"github.com/spf13/cobra"
)

// projectCmd represents the project command
var projectCmd = &cobra.Command{
	Use: "project",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var createCmd = &cobra.Command{
	Use: "create",
	Run: func(cmd *cobra.Command, args []string) {
		target, err := cmd.Flags().GetString("file")
		if err != nil {
			log.Fatalln(err)
		}
		newConf := project.ReadConfig(target)
		project.Save(args[0], newConf)
		fmt.Printf("Project %s created\n", target)
	},
	Args: cobra.ExactArgs(1),
}

var listCmd = &cobra.Command{
	Use: "list",
	Run: func(cmd *cobra.Command, args []string) {
		project.List()
	},
}

var deleteCmd = &cobra.Command{
	Use: "delete",
	Run: func(cmd *cobra.Command, args []string) {
		target, err := cmd.Flags().GetString("project")
		if err != nil {
			log.Fatalln(err)
		}
		project.Delete(target)
		fmt.Printf("Project %s deleted\n", target)
	},
}

func init() {
	//rootCmd.AddCommand(projectCmd)

	projectCmd.AddCommand(createCmd)
	createCmd.Flags().StringP("file", "f", "", "Target topology file")
	createCmd.MarkFlagRequired("file")

	projectCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringP("project", "p", "", "Project name")
	deleteCmd.MarkFlagRequired("project")

	projectCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// projectCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// projectCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
