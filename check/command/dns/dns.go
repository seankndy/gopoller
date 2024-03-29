package dns

import (
	"context"
	"errors"
	"fmt"
	"github.com/seankndy/gopoller/check"
	"net"
	"time"
)

type QueryType string

const (
	Host  QueryType = "Host"
	CNAME QueryType = "CNAME"
	MX    QueryType = "MX"
	TXT   QueryType = "TXT"
	PTR   QueryType = "PTR"
)

type Command struct {
	ServerIp      string
	ServerPort    uint16
	ServerTimeout time.Duration
	Query         string
	QueryType     QueryType

	Expected *[]string

	WarnRespTimeThreshold time.Duration
	CritRespTimeThreshold time.Duration
}

func (c *Command) Run(chk *check.Check) (*check.Result, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: c.ServerTimeout,
			}
			return d.DialContext(ctx, network, fmt.Sprintf("%s:%d", c.ServerIp, c.ServerPort))
		},
	}

	var resolvedEntries []string
	var err error
	startTime := time.Now()
	switch c.QueryType {
	case Host:
		chk.Debugf("sending Host request of %s to %s:%d", c.Query, c.ServerIp, c.ServerPort)
		resolvedEntries, err = r.LookupHost(context.Background(), c.Query)
	case CNAME:
		chk.Debugf("sending CNAME request of %s to %s:%d", c.Query, c.ServerIp, c.ServerPort)
		var name string
		name, err = r.LookupCNAME(context.Background(), c.Query)
		if err != nil {
			resolvedEntries = append(resolvedEntries, name)
		}
	case MX:
		chk.Debugf("sending MX request of %s to %s:%d", c.Query, c.ServerIp, c.ServerPort)
		var records []*net.MX
		records, err = r.LookupMX(context.Background(), c.Query)
		if err != nil {
			for _, mx := range records {
				resolvedEntries = append(resolvedEntries, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
			}
		}
	case TXT:
		chk.Debugf("sending TXT request of %s to %s:%d", c.Query, c.ServerIp, c.ServerPort)
		resolvedEntries, err = r.LookupTXT(context.Background(), c.Query)
	case PTR:
		chk.Debugf("sending PTR request of %s to %s:%d", c.Query, c.ServerIp, c.ServerPort)
		resolvedEntries, err = r.LookupAddr(context.Background(), c.Query)
	}

	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			if dnsErr.Timeout() {
				return check.NewResult(check.StateCrit, "CONNECTION_TIMEOUT", nil), nil
			}
		}

		return check.NewResult(check.StateUnknown, "CMD_FAILURE", nil), err
	}

	respTime := time.Now().Sub(startTime)
	respMs := float64(respTime.Microseconds()) / float64(time.Microsecond)

	chk.Debugf("resp=%.3f", respMs)

	resultMetrics := []check.ResultMetric{
		{
			Label: "resp",
			Value: fmt.Sprintf("%.3f", respMs),
			Type:  check.ResultMetricGauge,
		},
	}
	var resultState check.ResultState
	var resultReasonCode string

	expectedMatches := true
	if c.Expected != nil {
		for _, expectedVal := range *c.Expected {
			found := false
			for _, val := range resolvedEntries {
				if val == expectedVal {
					found = true
					break
				}
			}
			if !found {
				expectedMatches = false
				break
			}
		}
	}

	if !expectedMatches {
		resultState = check.StateCrit
		resultReasonCode = "UNEXPECTED_RESP"
	} else if respTime > c.CritRespTimeThreshold {
		resultState = check.StateCrit
		resultReasonCode = "RESP_TIME_EXCEEDED"
	} else if respTime > c.WarnRespTimeThreshold {
		resultState = check.StateWarn
		resultReasonCode = "RESP_TIME_EXCEEDED"
	} else {
		resultState = check.StateOk
		resultReasonCode = ""
	}

	return check.NewResult(resultState, resultReasonCode, resultMetrics), nil
}
