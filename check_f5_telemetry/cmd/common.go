package cmd

import (
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

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
	t, err := time.ParseDuration(timeout)
	if err != nil {
		return t, err
	}
	return t, nil
}