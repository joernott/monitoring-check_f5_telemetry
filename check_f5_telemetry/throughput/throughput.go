package throughput

import (
	"errors"
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
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

// Set of statistics
type ThroughputData struct {
	Timestamp time.Time  `yaml:"Timestamp" json:"Timestamp"`
	Fields    MetricData `yaml:"Fields" json:"Fields"`
}

//The Pool object created and initialized by NewPool consolidates the
// connection to Elasticsearch, the nagios object pool and index name
// needed to run the check.
type Throughput struct {
	index         string
	connection    *elasticsearch.Elasticsearch
	nagios        *nagiosplugin.Check
	Timestamp time.Time  `yaml:"Timestamp" json:"Timestamp"`
	Fields    MetricData `yaml:"Fields" json:"Fields"`

	FileName      string
	old           ThroughputData
	Current       ThroughputData
	Delta         ThroughputData
	Throughput    ThroughputData
	Duration      time.Duration
	ThroughputIn  float64
	ThroughputOut float64
}

// Creates a Throughput object containing the connection object to Elasticsearch, a
// Nagios object, the Index and statisticalData
func NewThroughput(Index string, FileName string, Connection *elasticsearch.Elasticsearch, Nagios *nagiosplugin.Check) (*Throughput, error) {
	var t *Throughput
	var err error

	logger := log.With().Str("func", "NewCheck").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")
	t = new(Throughput)
	t.index = Index
	t.connection = Connection
	t.nagios = Nagios
	t.FileName=FileName
	t.Current.Fields = make(MetricData)
	t.Delta.Fields = make(MetricData)
	t.Throughput.Fields = make(MetricData)
	t.old, err = readHistoricThroughput(t.FileName)
	if err != nil {
		t.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("%v. Could not read historic data", err))
		return nil, err
	}
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
	t.calculateThroughput()
	err = saveHistoricThroughput(t.FileName, t.Current)
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

	spew.Dump(e)
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
	t.Current.Timestamp = ts
	for _, f := range MetricFields {
		logger.Trace().Str("id", "DBG20030001").Str("field", f).Msg("Processing field")
		if fields["system.throughputPerformance."+f+".current"] != nil {
			t.Current.Fields[f] = fields["system.throughputPerformance."+f+".current"].([]interface{})[0].(float64)
		} else {
			logger.Warn().Str("id", "WRN2003001").
				Str("field", f).
				Msg("Field is missing")
		}
	}
	_, cbi_ok := t.Current.Fields["clientBitsIn"]
	if !cbi_ok {
		logger.Error().Str("id", "ERR2003003").Msg("Critical fields clientBitsIn is missing")
		t.nagios.AddResult(nagiosplugin.UNKNOWN, "Critical fields clientBitsIn is missing")
	}
	_, cbo_ok := t.Current.Fields["clientBitsOut"]
	if !cbo_ok {
		logger.Error().Str("id", "ERR2003004").Msg("Critical fields clientBitsOut is missing")
		t.nagios.AddResult(nagiosplugin.UNKNOWN, "Critical fields clientBitsOut is missing")
	}
	_, sbi_ok := t.Current.Fields["serverBitsIn"]
	if !sbi_ok {
		logger.Error().Str("id", "ERR2003003").Msg("Critical fields ServerBitsIn is missing")
		t.nagios.AddResult(nagiosplugin.UNKNOWN, "Critical fields ServerBitsIn is missing")
	}
	_, sbo_ok := t.Current.Fields["serverBitsOut"]
	if !sbo_ok {
		logger.Error().Str("id", "ERR2003003").Msg("Critical fields ServerBitsOut is missing")
		t.nagios.AddResult(nagiosplugin.UNKNOWN, "Critical fields ServerBitsOut is missing")
	}

	if !(cbi_ok && cbo_ok && sbi_ok && sbo_ok) {
		logger.Error().Str("id", "ERR2003003").Msg("One of the critical fields is missing, can't calculate throughput")
		spew.Dump(t.Current.Fields)
		return errors.New("One of the critical fields is missing, can't calculate throughput")
	}

	return nil
}

// calculate the Throughput
func (t *Throughput) calculateThroughput() {
	logger := log.With().Str("func", "calculateThroughput").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")

	t.Duration = t.Current.Timestamp.Sub(t.old.Timestamp)
	seconds := t.Duration.Seconds()

	// Calculating the delta between old and new values and througput per field
	for _, f := range MetricFields {
		t.Delta.Fields[f] = t.Current.Fields[f] - t.old.Fields[f]
		if seconds != 0 {
			t.Throughput.Fields[f] = t.Delta.Fields[f] / seconds
		} else {
			t.Throughput.Fields[f] = 0
		}
	}

	if seconds != 0 {
		t.ThroughputIn = (t.Delta.Fields["clientBitsIn"] + t.Delta.Fields["clientBitsOut"]) / seconds
		t.ThroughputOut = (t.Delta.Fields["serverBitsIn"] + t.Delta.Fields["serverBitsOut"]) / seconds
	} else {
		t.ThroughputIn = 0
		t.ThroughputOut = 0
	}
}

// Chech whether we have reached any thesholds
func (t *Throughput) Check(Warn string, Crit string, AgeWarn string, AgeCrit string) {
	logger := log.With().Str("func", "Check").Str("package", "throughput").Logger()
	logger.Trace().Msg("Enter func")

	ok := true
	if checkRange(t.nagios, Crit, t.ThroughputIn, "critical") {
		t.nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("CRITICAL: Throughput In %v is above critical threshold %v", t.ThroughputIn, Crit))
		ok = false
	}
	if checkRange(t.nagios, Crit, t.ThroughputOut, "critical") {
		t.nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("CRITICAL: Throughput Out %v is above critical threshold %v", t.ThroughputOut, Crit))
		ok = false
	}

	if checkRange(t.nagios, Warn, t.ThroughputIn, "warning") {
		t.nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("WARNING: Throughput In %v is above warning threshold %v", t.ThroughputIn, Warn))
		ok = false
	}
	if checkRange(t.nagios, Warn, t.ThroughputOut, "warning") {
		t.nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("WARNING: Throughput Out %v is above warning threshold %v", t.ThroughputOut, Warn))
		ok = false
	}

	if ok {
		t.nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("OK: Throughput In %v and Out %v are within Thtesholds %v/%v", t.ThroughputIn, t.ThroughputOut, Warn, Crit))
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
		p, _ := nagiosplugin.NewFloatPerfDatumValue(t.Throughput.Fields[f])
		t.nagios.AddPerfDatum("throughput_"+f, "", p, nil, nil, nil, nil)
		p, _ = nagiosplugin.NewFloatPerfDatumValue(t.Delta.Fields[f])
		t.nagios.AddPerfDatum("delta_"+f, "", p, nil, nil, nil, nil)
		p, _ = nagiosplugin.NewFloatPerfDatumValue(t.Current.Fields[f])
		t.nagios.AddPerfDatum(f, "c", p, nil, nil, nil, nil)
	}
	p, _ := nagiosplugin.NewFloatPerfDatumValue(t.ThroughputIn)
	t.nagios.AddPerfDatum("ThroughputIn", "", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(t.ThroughputOut)
	t.nagios.AddPerfDatum("ThroughputOut", "", p, nil, nil, nil, nil)
}
