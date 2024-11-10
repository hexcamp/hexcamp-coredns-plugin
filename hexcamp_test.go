package hexcamp

import (
	"bytes"
	"context"
	golog "log"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestExample(t *testing.T) {
	// Create a new Example Plugin. Use the test.ErrorHandler as the next plugin.
	x := HexCamp{Next: test.ErrorHandler()}
	domainName = "test.hex.camp"

	// Setup a new output buffer that is *not* standard output, so we can check if
	// example is really being printed.
	b := &bytes.Buffer{}
	golog.SetOutput(b)

	ctx := context.TODO()
	r := new(dns.Msg)
	r.SetQuestion("uxirkffr.test.hex.camp.", dns.TypeA)
	// Create a new Recorder that captures the result, this isn't actually used in this test
	// as it just serves as something that implements the dns.ResponseWriter interface.
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	// Call our plugin directly, and check the result.
	x.ServeDNS(ctx, rec, r)
	if len(rec.Msg.Answer) != 1 {
		t.Errorf("Wrong number of answers: %d", len(rec.Msg.Answer))

	}
	answer := rec.Msg.Answer[0]
	expected := "uxirkffr.test.hex.camp.	0	IN	CNAME	3.4.5.4.2.4.2.1.2.4.46.h3.test.hex.camp."
	if answer.String() != expected {
		t.Errorf("Unexpected answer: %s", answer.String())
	}
}
