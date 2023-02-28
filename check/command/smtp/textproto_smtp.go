package smtp

import (
	"fmt"
	"net"
	"net/textproto"
	"time"
)

type TextProtoSmtp struct {
	text *textproto.Conn
}

func (t *TextProtoSmtp) Connect(c *Command) error {
	dialer := net.Dialer{Timeout: c.Timeout}
	conn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%d", c.Addr, c.Port))
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(c.Timeout))

	t.text = textproto.NewConn(conn)
	_, _, err = t.text.ReadResponse(220)
	if err != nil {
		return &NotReadyErr{Cause: err}
	}

	return nil
}

func (t *TextProtoSmtp) Close() error {
	if t.text != nil {
		return t.text.Close()
	}
	return nil
}

func (t *TextProtoSmtp) Cmd(s string) (int, time.Duration, error) {
	startTime := time.Now()
	id, err := t.text.Cmd(s)
	if err != nil {
		return 0, 0, err
	}

	t.text.StartResponse(id)
	defer t.text.EndResponse(id)

	code, _, err := t.text.ReadResponse(-1)

	duration := time.Now().Sub(startTime)

	return code, duration, err
}
