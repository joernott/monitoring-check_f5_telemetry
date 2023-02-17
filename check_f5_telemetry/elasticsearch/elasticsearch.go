// package elasticsearch handles the interaction with elasticsearch
package elasticsearch

import (
	"time"

	"github.com/joernott/lra"
	"github.com/rs/zerolog/log"
)

// Handle the connection to Elasticsearch
type Elasticsearch struct {
	Connection *lra.Connection
}

// The generic Error response can be used when the actual data is irrelevant or
// only errors would be returned. It only consists of the field "error"
type ElasticsearchErrorResponse struct {
	Error ElasticsearchError `json:"error"`
}
// Elasticsearch error data returned when Elasticsearch run into an error
type ElasticsearchError struct {
	RootCause []ElasticsearchErrorRootCause `json:"root_cause"`
	Reason    string                        `json:"reason"`
	Resource  ElasticsearchErrorResource    `json:"resource"`
	IndexUUID string                        `json:"index_uuid"`
	Index     string                        `json:"index"`
}

// Part of the error response, the java root cause
type ElasticsearchErrorRootCause struct {
	Type      string                     `json:"type"`
	Reason    string                     `json:"reason"`
	Resource  ElasticsearchErrorResource `json:"resource"`
	IndexUUID string                     `json:"index_uuid"`
	Index     string                     `json:"index"`
}

// Error type and Id, part of the Elasticsearch error response
type ElasticsearchErrorResource struct {
	Type string `json:"type"`
	Id   string `json:"id"`
}

//Create a new elasticsearch connection. SSL, Host, Port, User and Password
// specify where and how to connect to and how to authenticate. If ValidateSSL
// is false, the certificate of the elasticsearch server won't be checked.
// Optionally, a proxy URL can be specified. Setting Socks expects the proxy to
// be a socks proxy. Timeout should be long enough for Elasticsearch to do the
// actual Search/Transaction.
func NewElasticsearch(SSL bool, Host string, Port int, User string, Password string, ValidateSSL bool, Proxy string, Socks bool, Timeout time.Duration) (*Elasticsearch, error) {
	var e *Elasticsearch

	logger := log.With().Str("func", "NewElasticsearch").Str("package", "elasticsearch").Logger()
	e = new(Elasticsearch)

	hdr := make(lra.HeaderList)
	hdr["Content-Type"] = "application/json"

	logger.Debug().
		Str("id", "DBG10010001").
		Str("host", Host).
		Int("port", Port).
		Str("user", User).
		Str("password", "*").
		Bool("validate_ssl", ValidateSSL).
		Str("proxy", Proxy).Bool("socks", Socks).
		Msg("Create connection")
	c, err := lra.NewConnection(SSL,
		Host,
		Port,
		"",
		User,
		Password,
		ValidateSSL,
		Proxy,
		Socks,
		hdr,
		Timeout)
	if err != nil {
		logger.Error().Str("id", "ERR10010001").Err(err).Msg("Failed to create connection")
		return nil, err
	}
	e.Connection = c
	return e, nil
}
