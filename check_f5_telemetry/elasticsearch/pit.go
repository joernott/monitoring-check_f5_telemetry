package elasticsearch

import (
	"github.com/rs/zerolog/log"
)

// The response from Elasticsearch when requesting a PIT. See
// https://www.elastic.co/guide/en/elasticsearch/reference/current/point-in-time-api.html
type ElasticsearchPitResponse struct {
	Id    string             `json:"id"`
	Error ElasticsearchError `json:"error"`
}

// The actual PIT data returned as part of the PIT API response
type ElasticsearchPit struct {
	Id        string `json:"id"`
	KeepAlive string `json:"keep_alive"`
}

// Get a Point in time for a given index. Index is the name of the index and
// KeepAlive the duration (number with unit s,m,h) for it. A PIT is needed
// to ensure consistent data across multiple searches.
func (e *Elasticsearch) Pit(Index string, KeepAlive string) (string, error) {
	var x []byte
	logger := log.With().Str("func", "Pit").Str("package", "elasticsearch").Logger()
	ResultJson := new(ElasticsearchPitResponse)
	endpoint := "/" + Index + "/_pit?keep_alive=" + KeepAlive

	logger.Debug().Str("id", "DBG10040001").Str("index", Index).Str("keepalive", KeepAlive).Str("endpoint", endpoint).Msg("Get Point In Time")

	err := e.Connection.PostJSON(endpoint, x,ResultJson)
	if err != nil {
		logger.Error().Str("id", "ERR10040002").Err(err).Str("reason", ResultJson.Error.Reason).Msg("PIT failed")
		return "", err
	}
	pit := ResultJson.Id
	logger.Info().Str("id", "INF10040001").Str("index", Index).Str("keepalive", KeepAlive).Str("pit", pit).Str("endpoint", endpoint).Msg("Successfully got a pit")
	return pit, nil
}

// Delete a Point In Time, freeing the lock in Elasticsearch
func (e *Elasticsearch) DeletePit(Pit string) error {
	logger := log.With().Str("func", "DeletePit").Str("package", "elasticsearch").Logger()
	ResultJson := new(ElasticsearchErrorResponse)
	endpoint := "/_pit"
	s := "{\"id\":\"" + Pit + "\"}"

	logger.Debug().Str("id", "DBG10050001").Str("pit", Pit).Str("endpoint", endpoint).Msg("Delete Point In Time")
	err := e.Connection.DeleteJSON(endpoint, []byte(s),ResultJson)
	if err != nil {
		logger.Error().Str("id", "ERR10050002").Err(err).Str("reason", ResultJson.Error.Reason).Msg("Delete PIT failed")
		return err
	}
	logger.Info().Str("id", "INF10050001").Str("pit", Pit).Str("endpoint", endpoint).Msg("Successfully deleted pit")
	return nil
}
