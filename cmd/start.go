package cmd

import (
	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/utils"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a network topology",
	Long: `Start a network topology using the provided configuration files.
Automatically creates Docker containers, network links and FRR configuration files.`,
	Run: func(cmd *cobra.Command, args []string) {
		newConf := getConfig(cmd, args)
		// setConfigDir(newConf.Name)
		if n, err := cmd.Flags().GetBool("no-generate"); err == nil {
			if !n {
				generateConfigs(newConf)
			}
		} else {
			utils.Fatalln(err)
		}
		links, err := cmd.Flags().GetString("links")
		if err != nil {
			utils.Fatalln(err)
		}
		if nopull, err := cmd.Flags().GetBool("no-pull"); err == nil {
			if !nopull {
				utils.PullImages()
			}
		} else {
			utils.Fatalln(err)
		}
		newConf.StartAll(links)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("project", "p", "", "Project name")
	startCmd.Flags().IntSliceVar(&config.ASOnly, "as", nil, "Start only specified AS")
	startCmd.Flags().String("links", "all", `Restrict which links should be applied (all, internal, external, none). Defaults to all.`)
	startCmd.Flags().Bool("no-generate", false, "Do not generate configuration files")
	startCmd.Flags().Bool("no-pull", false, "Do not pull docker image from DockerHub.")
}
