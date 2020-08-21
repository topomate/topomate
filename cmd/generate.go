package cmd

import (
	"github.com/rahveiz/topomate/frr"
	"github.com/rahveiz/topomate/project"
	"github.com/rahveiz/topomate/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate configuration files",
	Long: `Generate configurations files for FRRouting.
They are located in $HOME/topomate/<name> by default. If no name is specified, it uses "generated".`,
	Run: func(cmd *cobra.Command, args []string) {
		newConf := getConfig(cmd, args)
		// setConfigDir(newConf.Name)
		generateConfigs(newConf)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)
	generateCmd.Flags().StringP("project", "p", "", "Project name")
}

func setConfigDir(dirname string) {
	if dirname != "" {
		if dirname == "generated" {
			utils.Fatalln("Name \"generated\" not allowed (used by default).")
		}
		viper.Set("ConfigDir", utils.GetHome()+"/topomate/"+dirname)
	}
}

func generateConfigs(p *project.Project) {
	foo := frr.GenerateConfig(p)
	frr.WriteAll(foo)
}
