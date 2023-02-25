package smtp

import (
	"errors"
	"fmt"
	"github.com/seankndy/gopoller"
	"time"
)

type Client interface {
	Connect(*Command) error
	Close() error
	Cmd(string) (int, time.Duration, error)
}

type NotReadyErr struct {
	Cause error
}

func (e *NotReadyErr) Error() string {
	return "smtp not ready"
}

func (e *NotReadyErr) Unwrap() error {
	return e.Cause
}

type Command struct {
	client Client

	Addr    string
	Port    uint16
	Timeout time.Duration

	Send                 string // typically a "HELO" or "EHLO" command
	ExpectedResponseCode int    // typically 250

	WarnRespTimeThreshold time.Duration
	CritRespTimeThreshold time.Duration
}

func (c *Command) SetClient(client Client) {
	c.client = client
}

var (
	DefaultClient = &TextProtoSmtp{}
)

func (c *Command) Run(gopoller.Check) (result gopoller.Result, err error) {
	var client Client
	if c.client != nil {
		client = c.client
	} else {
		client = DefaultClient
	}

	err = client.Connect(c)
	if err != nil {
		var notReadyErr *NotReadyErr
		if errors.As(err, &notReadyErr) {
			client.Close()
			return *gopoller.NewResult(gopoller.StateCrit, "SMTP_NOT_READY", nil), err
		}

		return *gopoller.NewResult(gopoller.StateCrit, "CONNECTION_ERROR", nil), err
	}
	defer func() {
		errC := client.Close()
		if err != nil {
			err = errC
		}
	}()

	actualResponseCode, respTime, err := client.Cmd(c.Send)
	if err != nil {
		return *gopoller.MakeUnknownResult("CMD_FAILURE"), err
	}
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

	if actualResponseCode != c.ExpectedResponseCode {
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
