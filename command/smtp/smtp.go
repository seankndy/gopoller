package smtp

import (
	"errors"
	"fmt"
	"github.com/seankndy/gollector"
	"net"
	"net/textproto"
	"time"
)

type Command struct {
	conn                  net.Conn
	Addr                  string
	Port                  uint16
	Timeout               time.Duration
	WarnRespTimeThreshold time.Duration
	CritRespTimeThreshold time.Duration
	Send                  string
	ExpectedResponseCode  int
}

func (c *Command) Run(check gollector.Check) (result gollector.Result, err error) {
	dialer := net.Dialer{Timeout: c.Timeout}
	c.conn, err = dialer.Dial("tcp", fmt.Sprintf("%s:%d", c.Addr, c.Port))
	if err != nil {
		var netError net.Error
		if errors.As(err, &netError) && netError.Timeout() {
			return *gollector.NewResult(gollector.StateCrit, "CONNECTION_TIMEOUT", nil), nil
		}

		return *gollector.MakeUnknownResult("CMD_FAILURE"), nil
	}
	defer func() {
		errC := c.conn.Close()
		if err != nil {
			err = errC
		}
	}()

	text := textproto.NewConn(c.conn)
	_, _, err = text.ReadResponse(220)
	if err != nil {
		return *gollector.NewResult(gollector.StateCrit, "SMTP_NOT_READY", nil), nil
	}

	startTime := time.Now()

	id, err := text.Cmd(c.Send)
	text.StartResponse(id)
	_, _, err = text.ReadResponse(c.ExpectedResponseCode)
	text.EndResponse(id)

	respTime := time.Now().Sub(startTime)
	respMs := float64(respTime.Microseconds()) / float64(time.Microsecond)

	resultMetrics := []gollector.ResultMetric{
		{
			Label: "resp",
			Value: fmt.Sprintf("%.3f", respMs),
			Type:  gollector.ResultMetricGauge,
		},
	}
	var resultState gollector.ResultState
	var resultReasonCode string

	if err != nil {
		resultState = gollector.StateCrit
		resultReasonCode = "UNEXPECTED_RESP"
	} else if respTime > c.CritRespTimeThreshold {
		resultState = gollector.StateCrit
		resultReasonCode = "RESP_TIME_EXCEEDED"
	} else if respTime > c.WarnRespTimeThreshold {
		resultState = gollector.StateWarn
		resultReasonCode = "RESP_TIME_EXCEEDED"
	} else {
		resultState = gollector.StateOk
		resultReasonCode = ""
	}

	return *gollector.NewResult(resultState, resultReasonCode, resultMetrics), nil
}
