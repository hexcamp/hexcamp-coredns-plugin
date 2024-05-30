package hexcamp

import (
	"context"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/uber/h3-go/v4"

	"github.com/miekg/dns"
)

// Define log to be a logger with the plugin name in it. This way we can just use log.Info and
// friends to log.
var log = clog.NewWithPlugin("hexcamp")

type HexCamp struct {
	Next plugin.Handler
}

var regex *regexp.Regexp

func init() {
	regex, _ = regexp.Compile(`^([^.]+)\.test\.hex\.camp\.$`)
}

func (h HexCamp) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	if state.QType() == dns.TypeA || state.QType() == dns.TypeAAAA || state.QType() == dns.TypeCNAME {
		matches := regex.FindStringSubmatch(state.Name())
		if len(matches) == 2 {
			str := strings.ToUpper(matches[1])
			padding := ""
			switch len(str) % 8 {
			case 2:
				padding = "======"
			case 4:
				padding = "===="
			case 5:
				padding = "==="
			case 7:
				padding = "=="
			}
			str = str + padding
			data, err := base32.StdEncoding.DecodeString(str)
			if err != nil {
				fmt.Printf("Base32 decoding failed, %v\n", err)
			} else {
				hex := strings.ReplaceAll(fmt.Sprintf("8%-14s", hex.EncodeToString(data)), " ", "f")

				cell2 := h3.Cell(h3.IndexFromString(hex))
				resolution := cell2.Resolution()
				base := cell2.BaseCellNumber()
				target := ""
				for i := resolution; i > 0; i-- {
					parent := cell2.Parent(i)
					childPos := parent.ChildPos(i - 1)
					target = fmt.Sprintf("%s%d.", target, childPos)
				}
				target = fmt.Sprintf("%s%d.h3.test.hex.camp.", target, base)
				rr := new(dns.CNAME)
				rr.Hdr = dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeCNAME, Class: state.QClass()}
				// rr.Target = "3.4.5.4.2.4.2.1.2.4.46.h3.test.hex.camp."
				rr.Target = target

				a := new(dns.Msg)
				a.SetReply(r)
				a.Authoritative = true
				a.Extra = []dns.RR{rr}
				fmt.Printf("hexcamp: %v => %v\n", matches[1], target)

				w.WriteMsg(a)

				return 0, nil
			}
		}
	}

	// pw := NewResponsePrinter(w)

	// Export metric with the server label set to the current server handling the request.
	requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()

	// Call next plugin (if any).
	return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
}

// Name implements the Handler interface.
func (h HexCamp) Name() string { return "hexcamp" }

// ResponsePrinter wrap a dns.ResponseWriter and will write example to standard output when WriteMsg is called.
type ResponsePrinter struct {
	dns.ResponseWriter
}

// NewResponsePrinter returns ResponseWriter.
func NewResponsePrinter(w dns.ResponseWriter) *ResponsePrinter {
	return &ResponsePrinter{ResponseWriter: w}
}

// WriteMsg calls the underlying ResponseWriter's WriteMsg method and prints "example" to standard output.
func (r *ResponsePrinter) WriteMsg(res *dns.Msg) error {
	log.Info("hexcamp-plugin")
	return r.ResponseWriter.WriteMsg(res)
}
