package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"kubewatch/config"
)

// redisConfigCmd represents the websocket subcommand
var redisConfigCmd = &cobra.Command{
	Use:   "redis",
	Short: "specific redis configuration",
	Long:  `specific redis configuration`,
	Run: func(cmd *cobra.Command, args []string) {
		conf, err := config.New()
		if err != nil {
			logrus.Fatal(err)
		}
		url, err := cmd.Flags().GetString("url")
		if err == nil {
			if len(url) > 0 {
				conf.Handler.Redis.Url = url
			}
		} else {
			logrus.Fatal(err)
		}
		key, err := cmd.Flags().GetString("ke-key")
		if err == nil {
			if len(key) > 0 {
				conf.Handler.Redis.KeKey = key
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
	redisConfigCmd.Flags().StringP("url", "u", "", "Specify redis url")
	redisConfigCmd.Flags().StringP("ke-key", "k", "", "Specify redis kube event key")
}