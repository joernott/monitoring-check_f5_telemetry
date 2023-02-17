package cmd

import (
	"fmt"
	// "os"

	"github.com/joernott/nagiosplugin/v2"

	"github.com/joernott/monitoring-check_f5_telemetry/check_f5_telemetry/elasticsearch"
	"github.com/joernott/monitoring-check_f5_telemetry/check_f5_telemetry/pool"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The subcommand "check" to execute a check. This is called by Nagios/Icinga2
var poolCmd = &cobra.Command{
	Use:   "pool",
	Short: "Check pool",
	Long:  `Check F5 pool status in telemetry data stored in elasticsearch`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
		err := HandleConfigFile()
		if err != nil {
			fmt.Println("Config error")
			panic(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var p *pool.Pool

		nagios := nagiosplugin.NewCheck()
		nagios.SetVerbosity(nagiosplugin.VERBOSITY_MULTI_LINE)
		defer nagios.Finish()

		parsedTimeout, err := parseTimeout(viper.GetString("timeout"))
		if err != nil {
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not parse timeout")
			return
		}

		elasticsearch, err := elasticsearch.NewElasticsearch(
			viper.GetBool("ssl"),
			viper.GetString("host"),
			viper.GetInt("port"),
			viper.GetString("user"),
			viper.GetString("password"),
			viper.GetBool("validatessl"),
			viper.GetString("proxy"),
			viper.GetBool("socks"),
			parsedTimeout,
		)
		if err != nil {
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not create connection to Elasticsearch")
			log.Fatal().Err(err).Msg("Could not create connection to Elasticsearch")
			nagios.Finish()
			return
		}

		p, err = pool.NewPool(viper.GetString("index"), viper.GetString("pool"), viper.GetBool("ignore_disabled"), elasticsearch, nagios)
		if err != nil {
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not create pool check")
			log.Fatal().Err(err).Msg("Could not create pool check")
			nagios.Finish()
			return
		}
		result, err := p.Execute()
		if err != nil {
			return
		}
		p.Check(result, viper.GetString("warning"),
			viper.GetString("critical"),
			viper.GetString("age_warning"),
			viper.GetString("age_critical"))
		log.Info().Msg("Check finished successfully")
		nagios.Finish()
		return
	},
}
