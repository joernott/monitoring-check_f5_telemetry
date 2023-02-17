package throughput

import (
	"io/ioutil"
	"os"
	//"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// read the ThroughputData from the given file
func readHistoricThroughput(FileName string) (ThroughputData, error) {
	var data ThroughputData
	logger := log.With().Str("func", "readHistoricThroughput").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")

	logger.Debug().Str("id", "DBG20110001").Str("filename", FileName).Msg("Read historic data")
	f, err := ioutil.ReadFile(FileName)
	if err != nil {
		logger.Warn().Str("id", "WRN20110001").Err(err).Str("filename", FileName).Msg("Failed to read historic data, using empty defaults")
		return data,nil
	}
	err = yaml.Unmarshal(f, &data)
	if err != nil {
		log.Fatal().Str("id", "ERR20110002").Str("file", FileName).Err(err).Msg("Error unmarshalling yaml in historic data file")
		return data, err
	}
	return data, nil
}

// save the current data to be used as historic data on the next call
func saveHistoricThroughput(FileName string, Data ThroughputData) (error){
	logger := log.With().Str("func", "saveHistoricThroughput").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")

	yaml, err := yaml.Marshal(Data)
	if err != nil {
		logger.Error().Str("id", "ERR20120001").Str("filename", FileName).Err(err).Msg("Could marshal yaml")
		return err
	}

	f, err := os.Create(FileName)
	if err != nil {
		logger.Error().Str("id", "ERR20120002").Str("filename", FileName).Err(err).Msg("Could not create file")
		return err
	}
	defer f.Close()

	_, err = f.Write(yaml)
	if err != nil {
		logger.Error().Str("id", "ERR20120003").Str("filename", FileName).Err(err).Msg("Could not write to file")
		return err
	}
	logger.Debug().Str("id", "DBG20120001").Time("timestamp", Data.Timestamp).Str("filename", FileName).Msg("Wrote status file")
	return nil
}