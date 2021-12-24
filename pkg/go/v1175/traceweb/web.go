// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package traceweb

import (
	"bufio"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sync"

	_ "net/http/pprof" // Required to use pprof

	"github.com/xyctruth/profiler/pkg/go/v1175/trace"
)

var (
	traceFile string
)

func main() {
	traceFile = "./trace"
	res, err := parseTrace()
	if err != nil {
		dief("%v\n", err)
	}
	ranges = splitTrace(res)
	handlers := make(map[string]http.HandlerFunc)
	handlers["/"] = httpMain
	handlers["/mmus"] = httpMMU
	handlers["/mmuPlot"] = httpMMUPlot
	handlers["/mmuDetails"] = httpMMUDetails
	handlers["/usertasks"] = httpUserTasks
	handlers["/usertask"] = httpUserTask
	handlers["/userregions"] = httpUserRegions
	handlers["/userregion"] = httpUserRegion
	handlers["/trace"] = httpTrace
	handlers["/jsontrace"] = httpJsonTrace
	handlers["/jsontrace"] = httpJsonTrace
	handlers["/trace_viewer_html"] = httpTraceViewerHTML
	handlers["/webcomponents.min.js"] = webcomponentsJS
	handlers["/io"] = serveSVGProfile(pprofByGoroutine(computePprofIO))
	handlers["/block"] = serveSVGProfile(pprofByGoroutine(computePprofBlock))
	handlers["/syscall"] = serveSVGProfile(pprofByGoroutine(computePprofSyscall))
	handlers["/sched"] = serveSVGProfile(pprofByGoroutine(computePprofSched))
	handlers["/regionio"] = serveSVGProfile(pprofByRegion(computePprofIO))
	handlers["/regionblock"] = serveSVGProfile(pprofByRegion(computePprofBlock))
	handlers["/regionsyscall"] = serveSVGProfile(pprofByRegion(computePprofSyscall))
	handlers["/regionsched"] = serveSVGProfile(pprofByRegion(computePprofSched))
	handlers["/goroutines"] = httpGoroutines
	handlers["/goroutine"] = httpGoroutine
}

var ranges []Range

var loader struct {
	once sync.Once
	res  trace.ParseResult
	err  error
}

// parseEvents is a compatibility wrapper that returns only
// the Events part of trace.ParseResult returned by parseTrace.
func parseEvents() ([]*trace.Event, error) {
	res, err := parseTrace()
	if err != nil {
		return nil, err
	}
	return res.Events, err
}

func parseTrace() (trace.ParseResult, error) {
	loader.once.Do(func() {
		tracef, err := os.Open(traceFile)
		if err != nil {
			loader.err = fmt.Errorf("failed to open trace file: %v", err)
			return
		}
		defer tracef.Close()

		// Parse and symbolize.
		res, err := trace.Parse(bufio.NewReader(tracef), "")
		if err != nil {
			loader.err = fmt.Errorf("failed to parse trace: %v", err)
			return
		}
		loader.res = res
	})
	return loader.res, loader.err
}

// httpMain serves the starting page.
func httpMain(w http.ResponseWriter, r *http.Request) {
	if err := templMain.Execute(w, ranges); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var templMain = template.Must(template.New("").Parse(`
<html>
<body>
{{if $}}
	{{range $e := $}}
		<a href="{{$e.URL}}">View trace ({{$e.Name}})</a><br>
	{{end}}
	<br>
{{else}}
	<a href="/trace">View trace</a><br>
{{end}}
<a href="/goroutines">Goroutine analysis</a><br>
<a href="/io">Network blocking profile</a> (<a href="/io?raw=1" download="io.profile">⬇</a>)<br>
<a href="/block">Synchronization blocking profile</a> (<a href="/block?raw=1" download="block.profile">⬇</a>)<br>
<a href="/syscall">Syscall blocking profile</a> (<a href="/syscall?raw=1" download="syscall.profile">⬇</a>)<br>
<a href="/sched">Scheduler latency profile</a> (<a href="/sche?raw=1" download="sched.profile">⬇</a>)<br>
<a href="/usertasks">User-defined tasks</a><br>
<a href="/userregions">User-defined regions</a><br>
<a href="/mmu">Minimum mutator utilization</a><br>
</body>
</html>
`))

func dief(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}

var debugMemoryUsage bool

func init() {
	v := os.Getenv("DEBUG_MEMORY_USAGE")
	debugMemoryUsage = v != ""
}
