package throughput

import (
	"errors"
	"fmt"
	"time"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_f5_telemetry/check_f5_telemetry/elasticsearch"
	"github.com/joernott/nagiosplugin/v2"
	"github.com/rs/zerolog/log"
)

// These are the metrics we get from the LTM
var MetricFields = [...]string{
	"serverIn",
	"serverOut",
	"serverBitsIn",
	"serverBitsOut",
	"clientIn",
	"clientOut",
	"clientBitsIn",
	"clientBitsOut",
	"inPackets",
	"outPackets",
	"inBits",
	"outBits"}

// Where we store the metrics in a ThrougputData element
type MetricData map[string]float64

//The Pool object created and initialized by NewPool consolidates the
// connection to Elasticsearch, the nagios object pool and index name
// needed to run the check.
type Throughput struct {
	index      string
	connection *elasticsearch.Elasticsearch
	nagios     *nagiosplugin.Check
	Timestamp  time.Time  `yaml:"Timestamp" json:"Timestamp"`
	Fields     MetricData `yaml:"Fields" json:"Fields"`
}

// Creates a Throughput object containing the connection object to Elasticsearch, a
// Nagios object, the Index and statisticalData
func NewThroughput(Index string, Connection *elasticsearch.Elasticsearch, Nagios *nagiosplugin.Check) (*Throughput, error) {
	var t *Throughput

	logger := log.With().Str("func", "NewCheck").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")
	t = new(Throughput)
	t.index = Index
	t.connection = Connection
	t.nagios = Nagios
	t.Fields = make(MetricData)
	return t, nil
}

// Execute the query and calculate the data, then write the history file
func (t *Throughput) Execute() error {
	logger := log.With().Str("func", "Execute").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")
	query := "{\"size\":1,\"sort\":{\"@timestamp\":\"desc\"},\"query\":{\"match_all\":{}},\"fields\":[\"@timestamp\",\"system.throughputPerformance.*.current\"],\"_source\":false}"
	data, err := t.connection.Search(t.index, query)
	if err != nil {
		reason := ""
		if data != nil {
			reason = data.Error.Reason
		}
		logger.Error().Str("id", "ERR20020001").
			Str("query", query).
			Str("reason", reason).
			Err(err).
			Msg("Could not run search")
		t.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("%v. Could not run search on index %v. Query is %v", err, t.index, query))
		return err
	}
	err = t.gatherThroughputData(data)
	if err != nil {
		return err
	}
	return nil
}

// Convert the Elasticsearch data into our data structure
func (t *Throughput) gatherThroughputData(e *elasticsearch.ElasticsearchResult) error {
	var fields elasticsearch.HitElement
	logger := log.With().Str("func", "gatherPoolState").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")

	if len(e.Hits.Hits) == 0 {
		fields = make(elasticsearch.HitElement)
	} else {
		fields = e.Hits.Hits[0].Fields
	}

	if len(fields) == 0 {
		t.nagios.AddResult(nagiosplugin.UNKNOWN, "No data for throughput check")
		logger.Error().Str("id", "ERR20030001").
			Msg("No data for throughput check")
		return errors.New("No data for throughput check")
	}
	f := "2006-01-02T15:04:05.000Z"
	ts, err := time.Parse(f, fmt.Sprintf("%v", fields["@timestamp"].([]interface{})[0]))
	if err != nil {
		t.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("Could not parse @timestamp %v. ", ts))
		logger.Error().Str("id", "ERR20030002").
			Str("field", "@timestamp").
			Str("value", fmt.Sprintf("%v", fields["@timestamp"].([]interface{})[0])).
			Str("format", f).
			Err(err).
			Msg("Could not parse timestamp")
		return err
	}
	t.Timestamp = ts
	for _, f := range MetricFields {
		logger.Trace().Str("id", "DBG20030001").Str("field", f).Msg("Processing field")
		if fields["system.throughputPerformance."+f+".current"] != nil {
			t.Fields[f] = fields["system.throughputPerformance."+f+".current"].([]interface{})[0].(float64)
		} else {
			logger.Warn().Str("id", "WRN2003001").
				Str("field", f).
				Msg("Field is missing")
		}
	}
	_, bi_ok := t.Fields["inBits"]
	if !bi_ok {
		logger.Error().Str("id", "ERR2003003").Msg("Critical fields inBits is missing")
		t.nagios.AddResult(nagiosplugin.UNKNOWN, "Critical fields inBits is missing")
	}
	_, bo_ok := t.Fields["outBits"]
	if !bo_ok {
		logger.Error().Str("id", "ERR2003004").Msg("Critical fields outBits is missing")
		t.nagios.AddResult(nagiosplugin.UNKNOWN, "Critical fields outBits is missing")
	}
	if !(bi_ok && bo_ok) {
		logger.Error().Str("id", "ERR2003003").Msg("One of the critical fields is missing, can't calculate throughput")
		return errors.New("One of the critical fields is missing, can't calculate throughput")
	}

	return nil
}

// Chech whether we have reached any thesholds
func (t *Throughput) Check(Warn string, Crit string, AgeWarn string, AgeCrit string) {
	logger := log.With().Str("func", "Check").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")

	ok := true
	if checkRange(t.nagios, Crit, t.Fields["inBits"], "critical") {
		t.nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("CRITICAL: Bits In %v is above critical threshold %v", t.Fields["inBits"], Crit))
		ok = false
	}
	if checkRange(t.nagios, Crit, t.Fields["outBits"], "critical") {
		t.nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("CRITICAL: Bits Out %v is above critical threshold %v", t.Fields["outBits"], Crit))
		ok = false
	}

	if checkRange(t.nagios, Warn, t.Fields["inBits"], "warning") {
		t.nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("WARNING: Bits In %v is above warning threshold %v", t.Fields["inBits"], Warn))
		ok = false
	}
	if checkRange(t.nagios, Warn, t.Fields["outBits"], "warning") {
		t.nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("WARNING: Bits Out %v is above warning threshold %v", t.Fields["outBits"], Warn))
		ok = false
	}

	if ok {
		t.nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("OK: Bits In %v and Out %v are within Thtesholds %v/%v", t.Fields["inBits"], t.Fields["outBits"], Warn, Crit))
	}
	t.checkAddPerfdata()
}

// check, if a value has reached the theshold
func checkRange(nagios *nagiosplugin.Check, CheckRange string, Value float64, AlertType string) bool {
	logger := log.With().Str("func", "checkRange").Str("package", "pool").Logger()
	logger.Trace().Msg("Enter func")
	if CheckRange == "" {
		return false
	}
	r, err := nagiosplugin.ParseRange(CheckRange)
	if err != nil {
		logger.Error().Str("id", "ERR20060001").
			Str("field", AlertType).
			Str("range", CheckRange).
			Err(err).
			Msg("Error parsing range")
		nagios.AddResult(nagiosplugin.UNKNOWN, "error parsing "+AlertType+" range "+CheckRange)
	}
	return r.Check(Value)
}

// add performance data to the nagios output
func (t *Throughput) checkAddPerfdata() {
	for _, f := range MetricFields {
		p, _ := nagiosplugin.NewFloatPerfDatumValue(t.Fields[f])
		t.nagios.AddPerfDatum(f, "", p, nil, nil, nil, nil)
	}
}
