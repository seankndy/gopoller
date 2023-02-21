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
	Addr                  string
	Port                  uint16
	Timeout               time.Duration
	WarnRespTimeThreshold time.Duration
	CritRespTimeThreshold time.Duration
	Send                  string
	ReceiveRegex          string
}

func (c *Command) Run(check gollector.Check) (gollector.Result, error) {
	dialer := net.Dialer{Timeout: c.Timeout}
	conn, err := dialer.Dial("smtp", fmt.Sprintf("%s:%d", c.Addr, c.Port))
	if err != nil {
		var netError net.Error
		if errors.As(err, &netError) && netError.Timeout() {
			return *gollector.NewResult(gollector.StateCrit, "CONNECTION_TIMEOUT", nil), nil
		}

		return *gollector.MakeUnknownResult("CMD_FAILURE"), nil
	}

	text := textproto.NewConn(conn)
	_, _, err = text.ReadResponse(220)
	if err != nil {
		return *gollector.NewResult(gollector.StateCrit, "SMTP_NOT_READY", nil), nil
	}

	//text.Cmd()
}
