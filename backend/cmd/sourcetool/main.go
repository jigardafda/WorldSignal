// Command sourcetool builds, validates and seeds the global source catalog.
//
//	sourcetool catalog                 # print candidate coverage stats (no network)
//	sourcetool validate [flags]        # validate candidates, write a JSON report
//	sourcetool seed [flags]            # validate, then upsert passing sources into the DB
//
// Common flags: -only {all|curated|gnews|industry}, -limit N, -concurrency N,
// -out report.json, -db <DATABASE_URL>.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/sources"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sourcetool <catalog|validate|seed> [flags]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	only := fs.String("only", "all", "candidate set: all|curated|gnews|industry")
	limit := fs.Int("limit", 0, "limit number of candidates (0 = no limit)")
	concurrency := fs.Int("concurrency", 24, "parallel validations")
	out := fs.String("out", "", "write a JSON report to this path")
	from := fs.String("from", "", "seed from an existing JSON report instead of re-validating")
	dbURL := fs.String("db", os.Getenv("DATABASE_URL"), "database URL (seed)")
	_ = fs.Parse(os.Args[2:])

	cands := pick(*only)
	if *limit > 0 && *limit < len(cands) {
		cands = cands[:*limit]
	}

	switch cmd {
	case "catalog":
		printStats(sources.Summarize(cands))
	case "validate":
		results := run(cands, *concurrency)
		report(results, *out)
	case "seed":
		var results []sources.Result
		if *from != "" {
			results = loadReport(*from)
			fmt.Fprintf(os.Stderr, "loaded %d results from %s\n", len(results), *from)
		} else {
			results = run(cands, *concurrency)
			report(results, *out)
		}
		seed(*dbURL, results)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", cmd)
		os.Exit(2)
	}
}

func pick(only string) []sources.Candidate {
	switch only {
	case "curated":
		return sources.CuratedCandidates()
	case "gnews":
		return sources.GNewsCandidates()
	case "industry":
		return sources.IndustryCandidates()
	default:
		return sources.All()
	}
}

func run(cands []sources.Candidate, concurrency int) []sources.Result {
	cfg := sources.DefaultConfig()
	cfg.Concurrency = concurrency
	v := sources.NewValidator(cfg)
	fmt.Fprintf(os.Stderr, "validating %d candidates (concurrency=%d)...\n", len(cands), concurrency)
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	results := v.ValidateAll(ctx, cands)
	ok := 0
	for _, r := range results {
		if r.OK {
			ok++
		}
	}
	fmt.Fprintf(os.Stderr, "validated %d in %s — %d OK, %d failed\n",
		len(results), time.Since(start).Round(time.Second), ok, len(results)-ok)
	return results
}

func report(results []sources.Result, path string) {
	// Coverage of the passing set.
	var passing []sources.Candidate
	for _, r := range results {
		if r.OK {
			passing = append(passing, r.Candidate)
		}
	}
	printStats(sources.Summarize(passing))
	if path == "" {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "report:", err)
		return
	}
	defer func() { _ = f.Close() }()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		fmt.Fprintln(os.Stderr, "report:", err)
		return
	}
	fmt.Fprintf(os.Stderr, "wrote report → %s\n", path)
}

func loadReport(path string) []sources.Result {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open report:", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()
	var results []sources.Result
	if err := json.NewDecoder(f).Decode(&results); err != nil {
		fmt.Fprintln(os.Stderr, "decode report:", err)
		os.Exit(1)
	}
	return results
}

func seed(dbURL string, results []sources.Result) {
	if dbURL == "" {
		dbURL = "postgresql://worldsignal:worldsignal@localhost:5432/worldsignal?sslmode=disable"
	}
	ctx := context.Background()
	d, err := db.Connect(ctx, dbURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db connect:", err)
		os.Exit(1)
	}
	defer d.Close()
	if err := d.MigrateContent(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
	sum, err := sources.SeedValid(ctx, d, results)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Printf("seeded: %d inserted, %d updated, %d logs, %d skipped\n",
		sum.Inserted, sum.Updated, sum.Logs, sum.Skipped)
}

func printStats(s sources.Stats) {
	fmt.Printf("total candidates: %d\n", s.Total)
	printMap("by scope", s.ByScope)
	printMap("by region", s.ByRegion)
	printMap("by discovery source", s.BySource)
	fmt.Printf("distinct countries: %d | distinct languages: %d | distinct industries: %d\n",
		len(s.ByCountry), len(s.ByLanguage), len(s.ByIndustry))
}

func printMap(label string, m map[string]int) {
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for k, v := range m {
		if k == "" {
			k = "(none)"
		}
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	fmt.Printf("%s:\n", label)
	for _, p := range pairs {
		fmt.Printf("  %-22s %d\n", p.k, p.v)
	}
}
