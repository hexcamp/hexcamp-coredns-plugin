package hexcamp

import (
	"context"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/coredns/coredns/plugin"
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
}

func (h HexCamp) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	domainNameRegex := strings.ReplaceAll(domainName, ".", `\.`)
	// fmt.Printf("Jim domainNameRegex: %v\n", domainNameRegex)
	regex, _ = regexp.Compile(`^(.*\.)?([^.]+)\.` + domainNameRegex + `\.$`)

	if state.QType() == dns.TypeA || state.QType() == dns.TypeAAAA || state.QType() == dns.TypeCNAME {
		matches := regex.FindStringSubmatch(state.Name())
		if len(matches) == 3 {
			// fmt.Printf("Jim matches: %+v\n", matches)
			prefix := matches[1]
			str := strings.ToUpper(matches[2])
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
				fmt.Printf("Base32 decoding failed, %v: %v\n", matches[1], err)
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
				target = fmt.Sprintf("%s%s%d.h3.%s.", prefix, target, base, domainName)
				rr := new(dns.CNAME)
				rr.Hdr = dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeCNAME, Class: state.QClass()}
				rr.Target = target

				a := new(dns.Msg)
				a.SetReply(r)
				a.Authoritative = true
				a.Answer = []dns.RR{rr}
				log.Infof("hexcamp: %v%v => %v\n", matches[1], matches[2], target)

				err := w.WriteMsg(a)
				if err != nil {
					fmt.Printf("hexcamp WriteMsg error: %v\n", err)
				}

				return 0, nil
			}
		}
	}

	// pw := NewResponsePrinter(w)

	// Export metric with the server label set to the current server handling the request.
	// requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()

	// Call next plugin (if any).
	return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
}

// Name implements the Handler interface.
func (h HexCamp) Name() string { return "hexcamp" }
