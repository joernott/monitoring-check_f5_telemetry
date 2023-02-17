package elasticsearch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// Pagination data needed for a paginated search.
// See https://www.elastic.co/guide/en/elasticsearch/reference/current/paginate-search-results.html for more information.
type ElasticsearchQueryPagination struct {
	Pit         ElasticsearchPit         `json:"pit"`          // Elasticsearch Point In Time
	SearchAfter ElasticsearchSearchAfter `json:"search_after"` // Information from the last search to be fed to the next as starting point
	Size        uint                     `json:"size"`         // The maximum size of a page
}

// A paginated search repeats the same query on the same index, every time
// continuing, where the last query ended. This keeps track of the data needed
// for a paginated search.
type ElasticsearchPaginatedSearch struct {
	e          *Elasticsearch               // Link back to the elasticsearch connection
	Index      string                       // Name of the index
	Query      string                       // The actual query
	Pagination ElasticsearchQueryPagination // Pagination data from the previous run
	Results    []*ElasticsearchResult       // Results from every search
}

// The information returned for search after is a dynamic mix of data types
type ElasticsearchSearchAfter []interface{}

// Starts a paginated search. This is pretty much the same as a regular search
// but the query string must contain a _PAGINATION_ placeholder, where a
// different pagination information blurb will be inserted for every page.
func (e *Elasticsearch) StartPaginatedSearch(Index string, Query string) (*ElasticsearchPaginatedSearch, error) {
	logger := log.With().Str("func", "Search").Str("package", "elasticsearch").Logger()

	Search := new(ElasticsearchPaginatedSearch)
	Search.e = e
	Search.Index = Index
	Search.Query = Query
	Search.Pagination.Size = 1000
	Search.Pagination.Pit.KeepAlive = fmt.Sprintf("%vs", e.Connection.Timeout.Seconds())
	pit, err := e.Pit(Index, Search.Pagination.Pit.KeepAlive)
	if err != nil {
		return nil, err
	}
	Search.Pagination.Pit.Id = pit

	q := strings.Replace(Search.Query, "_PAGINATION_", "\"pit\":{\"id\":\""+pit+"\"},\"size\":1000", -1)
	logger.Debug().Str("id", "DBG10060001").Str("query", q).Int("pagination", len(Search.Results)).Msg("First paginated search")
	result, err := e.Search("", q)
	Search.Results = append(Search.Results, result)
	if err != nil {
		return nil, err
	}
	logger.Debug().Str("id", "DBG10060002").Str("old_pit", Search.Pagination.Pit.Id).Str("new_pit", result.PitId).Int("hits", len(result.Hits.Hits)).Msg("Run of first paginated search complete")
	if len(result.Hits.Hits) > 0 {
		Search.Pagination.SearchAfter = result.Hits.Hits[len(result.Hits.Hits)-1].Sort
	}
	Search.Pagination.Pit.Id = result.PitId
	return Search, nil
}

// Fetches the next page of a pagination
func (p *ElasticsearchPaginatedSearch) Next() error {
	logger := log.With().Str("func", "ElasticsearchPaginatedSearch.Next").Str("package", "elasticsearch").Logger()

	if len(p.Pagination.SearchAfter) == 0 {
		err := errors.New("Tried to continue after end of search")
		logger.Error().Str("id", "ERR10070001").Err(err).Msg("Failed to cross the border")
		return err
	}
	j, err := json.Marshal(p.Pagination)
	if err != nil {
		logger.Error().Str("id", "ERR10070002").Err(err).Msg("Marshal pagination failed")
		return err
	}
	pagination := string(j[1 : len(j)-1])
	q := strings.Replace(p.Query, "_PAGINATION_", pagination, -1)
	logger.Debug().Str("id", "DBG10070002").Str("query", q).Int("pagination", len(p.Results)).Msg("Paginated search")
	result, err := p.e.Search("", q)
	p.Results = append(p.Results, result)
	if err != nil {
		return err
	}
	logger.Debug().Str("id", "DBG10070003").Str("old_pit", p.Pagination.Pit.Id).Str("new_pit", result.PitId).Int("hits", len(result.Hits.Hits)).Msg("Run of paginated search complete")
	if len(result.Hits.Hits) > 0 {
		p.Pagination.SearchAfter = result.Hits.Hits[len(result.Hits.Hits)-1].Sort
	} else {
		var x []interface{}
		p.Pagination.SearchAfter = x
	}
	p.Pagination.Pit.Id = result.PitId
	return nil
}

// Close the paginated search by feeing the PIT in Elasticsearch
func (p *ElasticsearchPaginatedSearch) Close() error {
	return p.e.DeletePit(p.Pagination.Pit.Id)
}
