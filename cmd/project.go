/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/rahveiz/topomate/config"
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
		fmt.Println("create project called")
		newConf := config.ReadConfig(args[0])
		project.Save(newConf)
	},
}

var loadCmd = &cobra.Command{
	Use: "load",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("load project called")
	},
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
		fmt.Println("delete project called")
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(createCmd)
	projectCmd.AddCommand(loadCmd)
	projectCmd.AddCommand(deleteCmd)
	projectCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// projectCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// projectCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
