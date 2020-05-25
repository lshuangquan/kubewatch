package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"kubewatch/config"
)

// websocketConfigCmd represents the websocket subcommand
var websocketConfigCmd = &cobra.Command{
	Use:   "websocket",
	Short: "specific websocket configuration",
	Long:  `specific websocket configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		conf, err := config.New()
		if err != nil {
			logrus.Fatal(err)
		}

		url, err := cmd.Flags().GetString("url")
		if err == nil {
			if len(url) > 0 {
				conf.Handler.Websocket.Url = url
			}
		} else {
			logrus.Fatal(err)
		}

		if err = conf.Write(); err != nil {
			logrus.Fatal(err)
		}
	},
}

func init() {
	websocketConfigCmd.Flags().StringP("url", "u", "", "Specify websocket url")
}
