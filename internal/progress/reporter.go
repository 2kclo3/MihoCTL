package progress

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type Reporter struct {
	out     io.Writer
	label   string
	total   int64
	current atomic.Int64
	done    chan struct{}
	once    sync.Once
}

func New(out io.Writer, label string, total int64) *Reporter {
	if out == nil {
		out = io.Discard
	}
	r := &Reporter{
		out:   out,
		label: label,
		total: total,
		done:  make(chan struct{}),
	}
	go r.loop()
	return r
}

func (r *Reporter) Wrap(reader io.Reader) io.Reader {
	return &countingReader{
		reader:   reader,
		reporter: r,
	}
}

func (r *Reporter) Add(n int64) {
	r.current.Add(n)
}

func (r *Reporter) Finish() {
	r.once.Do(func() {
		close(r.done)
		r.print(true)
		fmt.Fprintln(r.out)
	})
}

func (r *Reporter) loop() {
	r.print(false)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.print(false)
		case <-r.done:
			return
		}
	}
}

func (r *Reporter) print(final bool) {
	current := r.current.Load()
	if final && r.total > 0 && current < r.total {
		current = r.total
	}

	if r.total > 0 {
		percent := float64(current) * 100 / float64(r.total)
		fmt.Fprintf(r.out, "\r%s %s/%s (%.1f%%)", r.label, formatBytes(current), formatBytes(r.total), percent)
		return
	}
	fmt.Fprintf(r.out, "\r%s %s", r.label, formatBytes(current))
}

type countingReader struct {
	reader   io.Reader
	reporter *Reporter
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.reporter.Add(int64(n))
	}
	return n, err
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	value := float64(n)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, name := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, name)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}
