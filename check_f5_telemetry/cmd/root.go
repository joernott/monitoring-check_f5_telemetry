package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Calling check_f5_telemetry without a subcommand will just output the
// generic help.
var rootCmd = &cobra.Command{
	Use:   "check_f5_telemetry",
	Short: "Check f5 telemetry data stored in elasticsearch",
	Long: `check_f5_telemetry checks f5 loadbalancer telemetry data generated by f5_telemetry and stored in an elasticsearch cluster.
See https://clouddocs.f5.com/products/extensions/f5-telemetry-streaming/latest/ for details on the f5_telemetry IApp.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
		err := HandleConfigFile()
		if err != nil {
			fmt.Println("Config error")
			panic(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		return
	},
}

// Global variable for cobra, storing the viper configuration file name
var ConfigFile string

// Global variable for cobra, one of the zerolog log levels (TRACE, DEBUG, INFO,
// WARN, ERROR, FATAL,PANIC). Trace produces an extreme amount of log data, use
// with care on a small dataset
var LogLevel string

// Global variable for cobra, name of the log file, "-" logs to stdout
var LogFile string

// Global variable for cobra, used in the check subcommand
var UseSSL bool

// Global variable for cobra, validate the SSL certificate (check subcommand)
var ValidateSSL bool

// Global variable for cobra, hostname or IP (check subcommand)
var Host string

// Global variable for cobra, port of Elasticsearch (check subcommand)
var Port int

// Global variable for cobra, User for connecting to  Elasticsearch (check subcommand)
var User string

// Global variable for cobra, Password for connecting to Elasticsearch (check subcommand)
var Password string

//Global variable for cobra, URL of a proxy (check subcommand)
var Proxy string

// Global variable for cobra, is the proxy a Socks proxy (check subcommand)
var ProxyIsSocks bool

// Global variable for cobra, timeout for the checks
var Timeout string

// Global variable for cobra, name of the index containinf the data
var Index string

// Global variable for cobra, name of the pool to check
var Pool string

// Global variable for cobra, Warning range
var Warn string

// Global variable for cobra, Critical range
var Crit string

// Global variable for cobra, Maximum data age for a warning
var AgeWarn string

// Global variable for cobra, Maximum data age for a critical alert
var AgeCrit string

// Global variable for cobra, Ignore disabled pool members
var IgnoreDisabled bool

// Run the checkcommand
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Initialize the various parameters and set defaults
func init() {
	rootCmd.PersistentFlags().StringVarP(&ConfigFile, "config", "c", "", "Configuration file")
	rootCmd.PersistentFlags().StringVarP(&LogLevel, "loglevel", "l", "WARN", "Log level")
	rootCmd.PersistentFlags().StringVarP(&LogFile, "logfile", "L", "/var/log/icinga2/check_f5_telemetry.log", "Log file (use - to log to stdout)")

	rootCmd.PersistentFlags().BoolVarP(&UseSSL, "ssl", "s", true, "Use SSL")
	rootCmd.PersistentFlags().BoolVarP(&ValidateSSL, "validatessl", "v", true, "Validate SSL certificate")
	rootCmd.PersistentFlags().StringVarP(&Host, "host", "H", "localhost", "Hostname of the server")
	rootCmd.PersistentFlags().IntVarP(&Port, "port", "P", 9200, "Network port")
	rootCmd.PersistentFlags().StringVarP(&User, "user", "u", "", "Username for Elasticsearch")
	rootCmd.PersistentFlags().StringVarP(&Password, "password", "p", "", "Password for the Elasticsearch user (consider using the env variable CLE_PASSWORD instead of passing it via commandline)")
	rootCmd.PersistentFlags().StringVarP(&Proxy, "proxy", "y", "", "Proxy (defaults to none)")
	rootCmd.PersistentFlags().BoolVarP(&ProxyIsSocks, "socks", "Y", false, "This is a SOCKS proxy")
	rootCmd.PersistentFlags().StringVarP(&Timeout, "timeout", "T", "2m", "Timeout understood by time.ParseDuration")
	rootCmd.PersistentFlags().StringVarP(&Warn, "warning", "W", "", "Warning range")
	rootCmd.PersistentFlags().StringVarP(&Crit, "critical", "C", "", "Critical range")
	rootCmd.PersistentFlags().StringVarP(&AgeWarn, "age_warning", "a", "5m", "Warn if data is older than this")
	rootCmd.PersistentFlags().StringVarP(&AgeCrit, "age_critical", "A", "15m", "Critical if data is older than this")
	rootCmd.PersistentFlags().StringVarP(&Index, "index", "I", "f5_telemetry", "Name of the index containing the f5 telemetry data")

	poolCmd.PersistentFlags().StringVarP(&Pool, "pool", "O", "", "Name of the pool object to check")
	poolCmd.PersistentFlags().BoolVarP(&IgnoreDisabled, "ignore_disabled", "i", false, "Ignore disabled members")

	rootCmd.AddCommand(poolCmd)
	rootCmd.AddCommand(throughputCmd)

	viper.SetDefault("loglevel", "WARN")
	viper.SetDefault("logfile", "/var/log/icinga2/check_f5_telemetry.log")
	viper.SetDefault("ssl", true)
	viper.SetDefault("validatessl", true)
	viper.SetDefault("host", "localhost")
	viper.SetDefault("port", 9200)
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")
	viper.SetDefault("proxy", "")
	viper.SetDefault("socks", false)
	viper.SetDefault("timeout", "2m")
	viper.SetDefault("warning", "")
	viper.SetDefault("critical", "")
	viper.SetDefault("age_warning", "5m")
	viper.SetDefault("age_critical", "15m")
	viper.SetDefault("index", "f5_telemetry")

	viper.SetDefault("pool", "")
	viper.SetDefault("ignore_disabled", "false")

	viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("loglevel"))
	viper.BindPFlag("logfile", rootCmd.PersistentFlags().Lookup("logfile"))
	viper.BindPFlag("ssl", rootCmd.PersistentFlags().Lookup("ssl"))
	viper.BindPFlag("validatessl", rootCmd.PersistentFlags().Lookup("validatessl"))
	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("user", rootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("proxy", rootCmd.PersistentFlags().Lookup("proxy"))
	viper.BindPFlag("socks", rootCmd.PersistentFlags().Lookup("socks"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
	viper.BindPFlag("warning", rootCmd.PersistentFlags().Lookup("warning"))
	viper.BindPFlag("critical", rootCmd.PersistentFlags().Lookup("critical"))
	viper.BindPFlag("age_warning", rootCmd.PersistentFlags().Lookup("age_warning"))
	viper.BindPFlag("age_critical", rootCmd.PersistentFlags().Lookup("age_critical"))
	viper.BindPFlag("index", rootCmd.PersistentFlags().Lookup("index"))

	viper.BindPFlag("pool", poolCmd.PersistentFlags().Lookup("pool"))
	viper.BindPFlag("ignore_disabled", poolCmd.PersistentFlags().Lookup("ignore_disabled"))

	viper.SetEnvPrefix("cf5")
	viper.BindEnv("password")
}

// Load the configuration file if the parameter is set.
func HandleConfigFile() error {
	logger := log.With().Str("func", "rootCmd.HandleConfigFile").Str("package", "cmd").Logger()
	if ConfigFile != "" {
		logger.Debug().Str("file", ConfigFile).Msg("Read config from " + ConfigFile)
		viper.SetConfigFile(ConfigFile)

		if err := viper.ReadInConfig(); err != nil {
			logger.Error().Err(err).Msg("Could not read config file")
			return err
		}
	}
	return nil
}

// Configure the logging.
func setupLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	var output io.Writer
	logfile := viper.GetString("logfile")
	if logfile == "-" {
		output = os.Stdout
	} else {
		output = &lumberjack.Logger{
			Filename:   logfile,
			MaxBackups: 10,
			MaxAge:     1,
			Compress:   true,
		}
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	switch strings.ToUpper(viper.GetString("loglevel")) {
	case "TRACE":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "DEBUG":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "INFO":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "WARN":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "ERROR":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "FATAL":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "PANIC":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		err := errors.New("Illegal log level " + LogLevel)
		log.Error().Str("id", "ERR00001").Err(err).Msg("")
		os.Exit(3)
	}
	log.Debug().Str("id", "DBG00001").Str("func", "setupLogging").Str("logfile", LogFile).Msg("Logging to " + LogFile)
}

// Parse the timeout string into a go duration
func parseTimeout(timeout string) (time.Duration, error) {
	logger := log.With().Str("func", "rootCmd.parseTimeout").Str("package", "cmd").Logger()
	t, err := time.ParseDuration(timeout)
	if err != nil {
		logger.Error().Err(err).Str("timeout", timeout).Msg("Could not parse timeout")
		return t, err
	}
	return t, nil
}
