package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/rahveiz/topomate/config"
	"github.com/rahveiz/topomate/project"
	"github.com/rahveiz/topomate/utils"
	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string
var vFlag bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "topomate",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.topomate.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.PersistentFlags().BoolVarP(&config.VFlag, "verbose", "v", false, "Display informations")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.SetDefault("MainDir", config.DefaultDir)
	viper.SetDefault("ProjectDir", config.DefaultProjectDir)
	viper.SetDefault("ConfigDir", config.DefaultConfigDir)
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".topomate" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".topomate")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func getTarget(cmd *cobra.Command, args []string) string {
	var target string
	if cmd.Flags().Changed("project") {
		var err error
		target, err = cmd.Flags().GetString("project")
		if err != nil {
			log.Fatalln(err)
		}
		return target
	}

	if len(args) == 0 {
		log.Fatalln("File or project not specified")
	}

	return args[0]
}

func getConfig(cmd *cobra.Command, args []string) *project.Project {
	if cmd.Flags().Changed("project") {
		var err error
		target, err := cmd.Flags().GetString("project")
		if err != nil {
			utils.Fatalln(err)
		}
		viper.Set("ConfigDir", viper.GetString("ConfigDir")+"/"+target)
		return project.ReadConfig(utils.GetDirectoryFromKey("ProjectDir", "") + "/" + target + ".yml")
	}

	if len(args) == 0 {
		log.Fatalln("File or project not specified")
	}

	return project.ReadConfig(args[0])
}
