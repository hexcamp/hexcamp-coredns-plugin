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
	fmt.Printf("Jim hexcamp ServeDNS\n")
	state := request.Request{W: w, Req: r}

	if state.QType() == dns.TypeA || state.QType() == dns.TypeAAAA {
		fmt.Printf("Jim hexcamp A AAAA: %v\n", state.Name())
		matches := regex.FindStringSubmatch(state.Name())
		if len(matches) == 2 {
			fmt.Printf("Jim match: %v\n", matches[1])
			str := strings.ToUpper(matches[1])
			data, err := base32.StdEncoding.DecodeString(str)
			if err == nil {
				hex := strings.ReplaceAll(fmt.Sprintf("8%-14s", hex.EncodeToString(data)), " ", "f")
				fmt.Printf("hex: %v\n", hex)

				cell2 := h3.Cell(h3.IndexFromString(hex))
				fmt.Printf("cell2 %s\n", cell2)
				resolution := cell2.Resolution()
				fmt.Printf("cell2 resolution %v\n", resolution)
				base := cell2.BaseCellNumber()
				for i := resolution; i > 0; i-- {
					parent := cell2.Parent(i)
					childPos := parent.ChildPos(i - 1)
					fmt.Printf("  %d: Parent %s %d: %d\n", i, parent.String(), parent.Resolution(), childPos)
				}
				fmt.Printf("cell2 base %v\n", base)
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
