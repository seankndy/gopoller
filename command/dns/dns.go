package dns

import (
	"context"
	"errors"
	"fmt"
	"github.com/seankndy/gopoller"
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

func (c *Command) Run(check gopoller.Check) (gopoller.Result, error) {
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
		resolvedEntries, err = r.LookupHost(context.Background(), c.Query)
	case CNAME:
		var name string
		name, err = r.LookupCNAME(context.Background(), c.Query)
		if err != nil {
			resolvedEntries = append(resolvedEntries, name)
		}
	case MX:
		var records []*net.MX
		records, err = r.LookupMX(context.Background(), c.Query)
		if err != nil {
			for _, mx := range records {
				resolvedEntries = append(resolvedEntries, fmt.Sprintf("%d %s", mx.Pref, mx.Host))
			}
		}
	case TXT:
		resolvedEntries, err = r.LookupTXT(context.Background(), c.Query)
	case PTR:
		resolvedEntries, err = r.LookupAddr(context.Background(), c.Query)
	}

	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			if dnsErr.Timeout() {
				return *gopoller.NewResult(gopoller.StateCrit, "CONNECTION_TIMEOUT", nil), nil
			}
		}

		return *gopoller.NewResult(gopoller.StateUnknown, "CMD_FAILURE", nil), err
	}

	respTime := time.Now().Sub(startTime)
	respMs := float64(respTime.Microseconds()) / float64(time.Microsecond)

	resultMetrics := []gopoller.ResultMetric{
		{
			Label: "resp",
			Value: fmt.Sprintf("%.3f", respMs),
			Type:  gopoller.ResultMetricGauge,
		},
	}
	var resultState gopoller.ResultState
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
		resultState = gopoller.StateCrit
		resultReasonCode = "UNEXPECTED_RESP"
	} else if respTime > c.CritRespTimeThreshold {
		resultState = gopoller.StateCrit
		resultReasonCode = "RESP_TIME_EXCEEDED"
	} else if respTime > c.WarnRespTimeThreshold {
		resultState = gopoller.StateWarn
		resultReasonCode = "RESP_TIME_EXCEEDED"
	} else {
		resultState = gopoller.StateOk
		resultReasonCode = ""
	}

	return *gopoller.NewResult(resultState, resultReasonCode, resultMetrics), nil
}
