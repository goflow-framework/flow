package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

var (
	baselinePath = flag.String("baseline", ".ci/bench_baseline.json", "path to baseline JSON file in repo")
	thresholdsPath = flag.String("thresholds", ".ci/bench_thresholds.json", "path to thresholds JSON file")
	reportPath = flag.String("report", ".ci/bench_report.json", "output report path")
)

var re = regexp.MustCompile(`^([^\s]+).*?([0-9]+\.?[0-9]*)\s+ns/op`) 

type Baseline struct {
	Benchmarks map[string]float64 `json:"benchmarks"`
	CreatedAt  string             `json:"created_at"`
}

func parseBench(reader io.Reader) (map[string]float64, error) {
	res := map[string]float64{}
	s := bufio.NewScanner(reader)
	for s.Scan() {
		line := s.Text()
		m := re.FindStringSubmatch(line)
		if len(m) == 3 {
			name := m[1]
			var ns float64
			_, err := fmt.Sscan(m[2], &ns)
			if err != nil {
				continue
			}
			res[name] = ns
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func loadThresholds(path string) (map[string]float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := map[string]float64{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeBaseline(path string, bm map[string]float64) error {
	bl := Baseline{Benchmarks: bm, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	b, err := json.MarshalIndent(bl, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func loadBaseline(path string) (*Baseline, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bl Baseline
	if err := json.Unmarshal(b, &bl); err != nil {
		return nil, err
	}
	return &bl, nil
}

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: benchcmp [flags] <bench-output-files...>")
		os.Exit(2)
	}
	// parse all provided bench output files
	cur := map[string]float64{}
	for _, p := range flag.Args() {
		f, err := os.Open(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open %s: %v\n", p, err)
			continue
		}
		m, err := parseBench(f)
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed parse %s: %v\n", p, err)
			continue
		}
		for k, v := range m {
			cur[k] = v
		}
	}

	// if baseline missing, write baseline and exit with code 2
	if _, err := os.Stat(*baselinePath); os.IsNotExist(err) {
		if err := writeBaseline(*baselinePath, cur); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write baseline: %v\n", err)
			os.Exit(3)
		}
		fmt.Fprintf(os.Stderr, "baseline written to %s\n", *baselinePath)
		// Also write an empty report
		r := map[string]interface{}{"status": "baseline_written", "benchmarks": cur}
		rr, _ := json.MarshalIndent(r, "", "  ")
		_ = os.WriteFile(*reportPath, rr, 0o644)
		os.Exit(2)
	}

	baseline, err := loadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load baseline: %v\n", err)
		os.Exit(3)
	}

	thresholds := map[string]float64{"default": 0.20}
	if t, err := loadThresholds(*thresholdsPath); err == nil {
		for k, v := range t {
			thresholds[k] = v
		}
	}

	report := map[string]interface{}{"status": "ok", "compare_at": time.Now().UTC().Format(time.RFC3339)}
	reportDetails := map[string]map[string]interface{}{}
	regressions := 0

	for name, base := range baseline.Benchmarks {
		curv, ok := cur[name]
		if !ok {
			// missing in current run; skip
			continue
		}
		th := thresholds["default"]
		if v, ok := thresholds[name]; ok {
			th = v
		}
		// percent increase allowed
		allowed := base * (1 + th)
		delta := (curv - base) / base
		status := "ok"
		if curv > allowed {
			status = "regressed"
			regressions++
		}
		reportDetails[name] = map[string]interface{}{"baseline_ns_per_op": base, "current_ns_per_op": curv, "delta": delta, "status": status, "allowed_ns_per_op": allowed}
	}

	report["details"] = reportDetails
	if regressions > 0 {
		report["status"] = "regression"
	}

	b, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(*reportPath, b, 0o644)

	if regressions > 0 {
		fmt.Fprintf(os.Stderr, "found %d regressions; report written to %s\n", regressions, *reportPath)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout, "no regressions detected")
	os.Exit(0)
}
