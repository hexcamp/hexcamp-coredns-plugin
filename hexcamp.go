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
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"
	"github.com/uber/h3-go/v4"

	"github.com/miekg/dns"
)

// Define log to be a logger with the plugin name in it. This way we can just use log.Info and
// friends to log.
var log = clog.NewWithPlugin("hexcamp")

type HexCamp struct {
	DomainName string
	Next       plugin.Handler
}

// Result is the result of a Lookup
type Result int

const (
	// Success is a successful lookup.
	Success Result = iota
	// NameError indicates a nameerror
	NameError
	// Delegation indicates the lookup resulted in a delegation.
	Delegation
	// NoData indicates the lookup resulted in a NODATA.
	NoData
	// ServerFailure indicates a server failure during the lookup.
	ServerFailure
)

var regex *regexp.Regexp

func init() {
}

func (h HexCamp) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	domainNameRegex := strings.ReplaceAll(h.DomainName, ".", `\.`)
	// log.Debugf("DomainNameRegex: %v\n", domainNameRegex)
	regex, _ = regexp.Compile(`^(.*\.)?([^.]+)\.` + domainNameRegex + `\.$`)

	// fmt.Printf("Jim hexcamp request state: %+v\n", state)
	if state.QType() == dns.TypeA || state.QType() == dns.TypeAAAA ||
		state.QType() == dns.TypeCNAME || state.QType() == dns.TypeTXT {
		matches := regex.FindStringSubmatch(state.Name())
		if len(matches) == 3 {
			// fmt.Printf("Jim matches: %+v\n", matches)
			prefix := matches[1]
			str := strings.ToUpper(matches[2])
			if str == "H3" {
				return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
			}
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
				// fmt.Printf("Base32 decoding failed, %v: %v\n", matches[1], err)
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
				target = fmt.Sprintf("%s%s%d.h3.%s.", prefix, target, base, h.DomainName)
				rr := new(dns.CNAME)
				rr.Hdr = dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeCNAME, Class: state.QClass()}
				rr.Target = target
				rrs := []dns.RR{rr}
				// From coredns/plugin/file/lookup.go
				if state.QType() != dns.TypeCNAME {
					// answer, ns, extra, result := z.Lookup(ctx, state, qname)
					targetName := rrs[0].(*dns.CNAME).Target
					lookupRRs, result := h.doLookup(ctx, state, targetName, state.QType())
					log.Infof("hexcamp: CNAME lookup %v, %v\n", lookupRRs, result)
					rrs = append(rrs, lookupRRs...)
				}

				a := new(dns.Msg)
				a.SetReply(r)
				a.Authoritative = true
				a.Answer = rrs
				log.Infof("hexcamp: %v%v => %v\n", matches[1], matches[2], target)

				err := w.WriteMsg(a)
				if err != nil {
					// fmt.Printf("hexcamp WriteMsg error: %v\n", err)
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

func (h *HexCamp) doLookup(ctx context.Context, state request.Request, target string, qtype uint16) ([]dns.RR, Result) {
	m, e := upstream.New().Lookup(ctx, state, target, qtype)
	if e != nil {
		return nil, ServerFailure
	}
	if m == nil {
		return nil, Success
	}
	if m.Rcode == dns.RcodeNameError {
		return m.Answer, NameError
	}
	if m.Rcode == dns.RcodeServerFailure {
		return m.Answer, ServerFailure
	}
	if m.Rcode == dns.RcodeSuccess && len(m.Answer) == 0 {
		return m.Answer, NoData
	}
	return m.Answer, Success
}
