package elasticsearch

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// The result of a Search
type ElasticsearchResult struct {
	PitId        string                       `json:"pit_id"`
	Took         int                          `json:"took"`
	TimedOut     bool                         `json:"timed_out"`
	Shards       ElasticsearchShardResult     `json:"_shards"`
	Hits         ElasticsearchHitResult       `json:"hits"`
	Error        ElasticsearchError           `json:"error"`
	Status       int                          `json:"status"`
	Aggregations map[string]AggregationResult `json:"aggregations"`
}

// Results per shard
type ElasticsearchShardResult struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

// The matching documents in the search result, this contains the actual hits
// as well as some statistics
type ElasticsearchHitResult struct {
	Total    ElasticsearchHitTotal  `json:"total"`
	MaxScore float64                `json:"max_score"`
	Hits     []ElasticsearchHitList `json:"hits"`
}

// Search result data from a matching document
type ElasticsearchHitList struct {
	Index  string        `json:"_index"`
	Type   string        `json:"_type"`
	Id     string        `json:"_id"`
	Score  float64       `json:"_score"`
	Source HitElement    `json:"_source"`
	Fields HitElement    `json:"fields"`
	Sort   []interface{} `json:"sort"`
}

// Statistical data for the HitResult
type ElasticsearchHitTotal struct {
	Value    int64  `json:"total"`
	Relation string `json:"relation"`
}

// Data from an aggregation
type AggregationResult map[string]interface{}

// Hit elements match the documents data structure
type HitElement map[string]interface{}

// Conduct a search on the given Index using the provided Query. See
// https://www.elastic.co/guide/en/elasticsearch/reference/current/search-search.html
func (e *Elasticsearch) Search(Index string, Query string) (*ElasticsearchResult, error) {
	var ResultJson *ElasticsearchResult

	logger := log.With().Str("func", "Search").Str("package", "elasticsearch").Logger()
	ResultJson = new(ElasticsearchResult)
	endpoint := "/_search"
	if len(Index) > 0 {
		endpoint = "/" + Index + "/_search"
	}

	logger.Debug().Str("id", "DBG10020001").Str("query", Query).Str("endpoint", endpoint).Msg("Execute Query")
	err:=e.Connection.PostJSON(endpoint, []byte(Query), ResultJson)
	if err != nil {
		logger.Error().Str("id", "ERR10020002").Err(err).Msg("Query failed")
		return ResultJson, err
	}
	logger.Info().Str("id", "INF10020001").Str("query", Query).Str("endpoint", endpoint).Msg("Successfully executed query")
	return ResultJson, nil
}

// Retrieve an element from a HitResult, which may be a nested structure.
// Needle is the name of the element to retrieve. For nested fields, the dot
// notation can be used, e.g. "log.level" fetches the contents of "level" from
// the "log" element.
func (haystack HitElement) Get(Needle string) (interface{}, bool) {
	if len(haystack) == 0 {
		return "", false
	}
	n := strings.Split(Needle, ".")
	key := n[0]
	if len(n) > 1 {
		subkeys := strings.Join(n[1:], ".")
		subvalues, ok := haystack[n[0]].(HitElement)
		if !ok {
			return "", ok
		}
		return subvalues.Get(subkeys)
	}
	value, ok := haystack[key]
	if !ok {
		return "", ok
	}
	return value, true
}

// Retrieve a String from a HitResult, which may be a nested structure.
// Needle is the name of the element to retrieve. For nested fields, the dot
// notation can be used, e.g. "log.level" fetches the contents of "level" from
// the "log" element.
func (haystack HitElement) GetString(Needle string) (string, bool) {
	s, ok := haystack.Get(Needle)
	return fmt.Sprintf("%v", s), ok
}
