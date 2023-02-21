package pool

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"regexp"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_f5_telemetry/check_f5_telemetry/elasticsearch"
	"github.com/joernott/nagiosplugin/v2"

	"github.com/rs/zerolog/log"
)

//The Pool object created and initialized by NewPool consolidates the
// connection to Elasticsearch, the nagios object pool and index name
// needed to run the check.
type Pool struct {
	index           string
	pool            string
	ignore_disabled bool
	connection      *elasticsearch.Elasticsearch
	nagios          *nagiosplugin.Check
}

// Pool member data
type PoolMemberData struct {
	AvailabilityState string
	EnabledState      string
}

// States of the various pool members
type PoolMemberState map[string]PoolMemberData

// Consolidated states of the pool
type PoolState struct {
	AvailabilityState   string
	Members             PoolMemberState
	Timestamp           time.Time
	CurrentConnections  float64
	MaxConnections      float64
	PacketsIn           float64
	PacketsOut          float64
	BitsIn              float64
	BitsOut             float64
	ActiveMemberCount   uint
	DownMemberCount     uint
	DisabledMemberCount uint
	UnavailableMembers  uint
	TotalMembers        uint
}

// Creates a Pool object containing the connection object to Elasticsearch, a
// Nagios object, the Index and pool name
func NewPool(Index string, PoolName string, IgnoreDisabled bool, Connection *elasticsearch.Elasticsearch, Nagios *nagiosplugin.Check) (*Pool, error) {
	var p *Pool

	logger := log.With().Str("func", "NewCheck").Str("package", "pool").Logger()
	logger.Trace().Msg("Enter func")
	p = new(Pool)
	p.index = Index
	p.pool = PoolName
	p.ignore_disabled = IgnoreDisabled
	p.connection = Connection
	p.nagios = Nagios

	return p, nil
}

// Execute the query
func (p *Pool) Execute() (*PoolState, error) {
	logger := log.With().Str("func", "Execute").Str("package", "pool").Logger()
	logger.Trace().Msg("Enter func")
	q := strings.ReplaceAll("{\"size\":1,\"sort\":{\"@timestamp\":\"desc\"},\"query\":{\"match_all\":{}},\"fields\":[\"@timestamp\",\"pools._POOL_.*\"],\"_source\":false}", "_POOL_", p.pool)
	data, err := p.connection.Search(p.index, q)
	if err != nil {
		reason := ""
		if data != nil {
			reason = data.Error.Reason
		}
		logger.Error().Str("id", "ERR10020001").
			Str("parsed_query", q).
			Str("reason", reason).
			Err(err).
			Msg("Could not run search")
		p.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("%v. Could not run search on index %v. Query is %v", err, p.index, q))
		return nil, err
	}
	s, err := p.gatherPoolState(data)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// Convert the Elasticsearch data into our data structure
func (p *Pool) gatherPoolState(e *elasticsearch.ElasticsearchResult) (*PoolState, error) {
	var s *PoolState
	var fields elasticsearch.HitElement
	logger := log.With().Str("func", "gatherPoolState").Str("package", "pool").Str("pool", p.pool).Logger()
	logger.Trace().Msg("Enter func")
	if len(e.Hits.Hits) == 0 {
		fields = make(elasticsearch.HitElement)
	} else {
		fields = e.Hits.Hits[0].Fields
	}

	if len(fields) == 0 {
		p.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("No data for pool %v. ", p.pool))
		logger.Error().Str("id", "ERR10030001").Msg("No data for pool")
		return nil, errors.New("No data for pool " + p.pool)
	}
	s = new(PoolState)
	fieldname := "pools." + p.pool + ".availabilityState.keyword"
	if fields[fieldname] == nil {
		p.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("No availabilityState for pool %v. Does this pool exist?", p.pool))
		logger.Error().Str("id", "ERR10030002").Str("field",fieldname).Msg("No availabilityState for pool")
		return nil, errors.New("No availabilityState for pool " + p.pool)
	}
	s.AvailabilityState = fmt.Sprintf("%v", fields[fieldname].([]interface{})[0])
	f := "2006-01-02T15:04:05.000Z"
	t := fmt.Sprintf("%v", fields["@timestamp"].([]interface{})[0])
	ts, err := time.Parse(f, t)
	if err != nil {
		p.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("Could not parse @timestamp %v. ", t))
		logger.Error().Str("id", "ERR10030003").
			Str("field", "@timestamp").
			Str("value", t).
			Str("format", f).
			Err(err).
			Msg("Could not parse timestamp")
		return nil, err
	}
	s.Timestamp = ts
	if s.CurrentConnections,err = p.getField(fields,"serverside.curConns"); err != nil {
		return nil, err
	}
	if s.MaxConnections,err = p.getField(fields,"serverside.maxConns"); err != nil {
		return nil, err
	}
	if s.PacketsIn,err = p.getField(fields,"serverside.pktsIn"); err != nil {
		return nil, err
	}
	if s.PacketsOut,err = p.getField(fields,"serverside.pktsOut"); err != nil {
		return nil, err
	}
	if s.BitsIn,err = p.p.getField(fields,"serverside.bitsIn"); err != nil {
		return nil, err
	}
	if s.BitsOut,err = p.getField(fields,"serverside.bitsOut"); err != nil {
		return nil, err
	}
	amc, err:=p.getField(fields,"activeMemberCnt")
	if err != nil {
		return nil, err
	}
	s.ActiveMemberCount = uint(amc)
	s.Members = make(PoolMemberState)
	for f, _ := range fields {
		r := "pools\\." + p.pool + ".members\\..*\\.enabledState.keyword"
		match, err := regexp.MatchString(r, f)
		if err != nil {
			logger.Error().Str("id", "ERR10030004").
				Str("field", f).
				Str("regex", r).
				Err(err).
				Msg("Could not match regex")
			return nil, err
		}
		if match {
			n := strings.Split(f, ".")
			member := n[3]
			e := fmt.Sprintf("%v", fields["pools."+p.pool+".members."+member+".enabledState.keyword"].([]interface{})[0])
			if e != "enabled" {
				s.DisabledMemberCount++
			}
			a := fmt.Sprintf("%v", fields["pools."+p.pool+".members."+member+".availabilityState.keyword"].([]interface{})[0])
			if a != "available" {
				s.DownMemberCount++
			}
			if p.ignore_disabled {
				if a != "available" || e != "enabled" {
					s.UnavailableMembers++
				}
			} else {
				if e == "enabled" && a != "available" {
					s.UnavailableMembers++
				}
			}
			m := PoolMemberData{a, e}
			s.TotalMembers++
			logger.Debug().Str("id", "DBG10030001").
				Bool("match", true).
				Str("field", f).
				Str("regex", r).
				Str("member", member).
				Str("availabilityState", a).
				Str("enabledState", e).
				Msg("Match found")
			s.Members[member] = m
		} else {
			logger.Trace().Str("id", "DBG10030002").
				Bool("match", false).
				Str("field", f).
				Str("regex", r).
				Msg("NoMatch")
		}
	}
	return s, nil
}

func (p *Pool)getField(fields elasticsearch.HitElement, fieldname string)(float64,error) {
	logger := log.With().Str("func", "gatherPoolState").Str("package", "pool").Str("pool", p.pool).Logger()
	logger.Trace().Msg("Enter func")
	if fields[fieldname] == nil {
		p.nagios.AddResult(nagiosplugin.UNKNOWN, fmt.Sprintf("No field %v for pool %v. Does this pool exist?", fieldname, p.pool))
		logger.Error().Str("id", "ERR10040002").Str("field",fieldname).Msg("No availabilityState for pool")
		return 0, errors.New(fmt.Sprintf("No field %v for pool %v. Does this pool exist?", fieldname, p.pool))
	}
	return fields["pools."+p.pool+"."+fieldname].([]interface{})[0].(float64), nil
}

func (p *Pool) Check(s *PoolState, Warn string, Crit string, AgeWarn string, AgeCrit string) {
	logger := log.With().Str("func", "Check").Str("package", "pool").Logger()
	logger.Trace().Msg("Enter func")

	ok := true
	if checkRange(p.nagios, Crit, s.UnavailableMembers, "critical") {
		p.nagios.AddResult(nagiosplugin.CRITICAL, fmt.Sprintf("CRITICAL: %v of %v pool members unavailable", s.UnavailableMembers, s.TotalMembers))
		ok = false
	}
	if checkRange(p.nagios, Warn, s.UnavailableMembers, "warning") {
		p.nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("WARNING: %v of %v pool members unavailable", s.UnavailableMembers, s.TotalMembers))
		ok = false
	}
	if ok {
		p.nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("OK: pool %v is healthy, %v members available", p.pool, s.ActiveMemberCount))
	}
	checkAddMemberResults(p.nagios, s.Members, p.ignore_disabled)
	checkAddPerfdata(p.nagios, s)
}

func checkRange(nagios *nagiosplugin.Check, CheckRange string, Value uint, AlertType string) bool {
	logger := log.With().Str("func", "checkRange").Str("package", "pool").Logger()
	logger.Trace().Msg("Enter func")
	if CheckRange == "" {
		return false
	}
	r, err := nagiosplugin.ParseRange(CheckRange)
	if err != nil {
		logger.Error().Str("id", "ERR10060001").
			Str("field", AlertType).
			Str("range", CheckRange).
			Err(err).
			Msg("Error parsing range")
		nagios.AddResult(nagiosplugin.UNKNOWN, "error parsing "+AlertType+" range "+CheckRange)
	}
	return r.Check(float64(Value))
}

func checkAddMemberResults(nagios *nagiosplugin.Check, members PoolMemberState, ignore_disabled bool) {
	for member, status := range members {
		if ignore_disabled {
			if status.AvailabilityState != "available" || status.EnabledState != "enabled" {
				nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("Member %v: %v, %v", member, status.EnabledState, status.AvailabilityState))
			} else {
				nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("Member %v: %v, %v", member, status.EnabledState, status.AvailabilityState))
			}
		} else {
			if status.EnabledState == "enabled" && status.AvailabilityState != "available" {
				nagios.AddResult(nagiosplugin.WARNING, fmt.Sprintf("Member %v: %v, %v", member, status.EnabledState, status.AvailabilityState))
			} else {
				nagios.AddResult(nagiosplugin.OK, fmt.Sprintf("Member %v: %v, %v", member, status.EnabledState, status.AvailabilityState))
			}
		}
	}
}

func checkAddPerfdata(nagios *nagiosplugin.Check, s *PoolState) {
	p, _ := nagiosplugin.NewFloatPerfDatumValue(s.CurrentConnections)
	nagios.AddPerfDatum("current_connections", "", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(s.MaxConnections)
	nagios.AddPerfDatum("max_connections", "", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(s.PacketsIn)
	nagios.AddPerfDatum("packets_in", "c", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(s.PacketsOut)
	nagios.AddPerfDatum("packets_out", "c", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(s.BitsIn)
	nagios.AddPerfDatum("bits_in", "c", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(s.BitsOut)
	nagios.AddPerfDatum("bits_out", "c", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(float64(s.ActiveMemberCount))
	nagios.AddPerfDatum("active_member_count", "", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(float64(s.DownMemberCount))
	nagios.AddPerfDatum("down_member_count", "", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(float64(s.UnavailableMembers))
	nagios.AddPerfDatum("unavailable_member_count", "", p, nil, nil, nil, nil)
	p, _ = nagiosplugin.NewFloatPerfDatumValue(float64(s.TotalMembers))
	nagios.AddPerfDatum("total_members", "", p, nil, nil, nil, nil)
}
