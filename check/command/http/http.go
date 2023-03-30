package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/seankndy/gopoller/check"
	"net/http"
	"strings"
	"time"
)

// Command is a check.Command that makes an HTTP request and verifies the response code while also measuring response
// time.
type Command struct {
	ReqUrl        string
	ReqMethod     string
	ReqTimeout    time.Duration
	ReqBody       string
	SkipSslVerify bool

	ExpectedResponseCode  int
	WarnRespTimeThreshold time.Duration
	CritRespTimeThreshold time.Duration
}

func (c *Command) Run(chk *check.Check) (*check.Result, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.SkipSslVerify},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), c.ReqTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, c.ReqMethod, c.ReqUrl, strings.NewReader(c.ReqBody))
	if err != nil {
		return check.MakeUnknownResult("CMD_FAILURE"), err
	}

	chk.Debugf("sending %s request to %s with body %s", c.ReqMethod, c.ReqUrl, c.ReqBody)
	startTime := time.Now()
	response, err := client.Do(request)
	respTime := time.Now().Sub(startTime)
	if err != nil {
		var tlsVerifyErr *tls.CertificateVerificationError
		if errors.Is(err, context.DeadlineExceeded) {
			return check.NewResult(check.StateCrit, "CONNECTION_ERROR", nil), err
		} else if errors.As(err, &tlsVerifyErr) {
			return check.NewResult(check.StateCrit, "HTTP_SSL_FAILURE", nil), err
		}

		return check.MakeUnknownResult("CMD_FAILURE"), err
	}
	defer response.Body.Close()

	resultMetrics := []check.ResultMetric{
		{
			Label: "resp",
			Value: fmt.Sprintf("%.3f", float64(respTime.Microseconds())/float64(time.Microsecond)),
			Type:  check.ResultMetricGauge,
		},
	}
	var resultState check.ResultState
	var resultReasonCode string

	if response.StatusCode != c.ExpectedResponseCode {
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
