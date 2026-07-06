// Command sourcetool builds, validates and seeds the global source catalog.
//
//	sourcetool catalog                 # print candidate coverage stats (no network)
//	sourcetool validate [flags]        # validate candidates, write a JSON report
//	sourcetool seed [flags]            # validate, then upsert passing sources into the DB
//	sourcetool recategorize [flags]    # in-place reclassify GENERAL/uncategorized signals (heuristic)
//
// Common flags: -only {all|curated|gnews|industry}, -limit N, -concurrency N,
// -out report.json, -db <DATABASE_URL>. recategorize adds -apply (default dry-run).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/llm"
	"github.com/worldsignal/backend/internal/sources"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: sourcetool <catalog|validate|seed|recategorize> [flags]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	only := fs.String("only", "all", "candidate set: all|curated|gnews|industry")
	limit := fs.Int("limit", 0, "limit number of candidates/signals (0 = no limit)")
	concurrency := fs.Int("concurrency", 24, "parallel validations")
	out := fs.String("out", "", "write a JSON report to this path")
	from := fs.String("from", "", "seed from an existing JSON report instead of re-validating")
	dbURL := fs.String("db", os.Getenv("DATABASE_URL"), "database URL (seed)")
	apply := fs.Bool("apply", false, "recategorize: persist changes (default dry-run)")
	useLLM := fs.Bool("llm", false, "recategorize: use the OpenAI gateway (handles non-English); falls back to heuristic per-signal on failure")
	_ = fs.Parse(os.Args[2:])

	if cmd == "recategorize" {
		recategorize(*dbURL, *limit, *apply, *useLLM, *concurrency)
		return
	}

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

// recategorize reclassifies every signal currently in the GENERAL domain (or with
// no category) in place, using the deterministic heuristic classifier on the
// signal's own title+summary — no network, no LLM. Dry-run by default: it prints
// the resulting distribution and how many signals leave GENERAL; pass -apply to
// persist (updates eventType and the category attribute per signal).
func recategorize(dbURL string, limit int, apply, useLLM bool, concurrency int) {
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

	var gw llm.Gateway
	if useLLM {
		key, model := os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_MODEL")
		if key == "" {
			fmt.Fprintln(os.Stderr, "-llm set but OPENAI_API_KEY is empty (source backend/.env.local)")
			os.Exit(1)
		}
		gw = llm.NewOpenAIGateway(key, model)
		fmt.Fprintf(os.Stderr, "using LLM gateway (model=%s, concurrency=%d)\n", model, concurrency)
	}

	sigs, err := d.UncategorizedSignalTexts(ctx, limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query uncategorized:", err)
		os.Exit(1)
	}
	if len(sigs) == 0 {
		fmt.Println("no GENERAL/uncategorized signals to recategorize")
		return
	}
	mode := "heuristic"
	if useLLM {
		mode = "LLM"
	}
	fmt.Fprintf(os.Stderr, "classifying %d GENERAL/uncategorized signals (%s)...\n", len(sigs), mode)

	// classify returns the primary code + all category tags for one signal.
	classify := func(s db.SignalText) []db.CategoryTag {
		body := s.Summary + " " + s.Body
		if useLLM {
			r := llm.EnrichArticle(ctx, gw, llm.EnrichInput{Title: s.Title, Body: body})
			out := make([]db.CategoryTag, 0, len(r.Tags))
			for _, t := range r.Tags {
				out = append(out, db.CategoryTag{Code: t.Code, Confidence: t.Confidence})
			}
			if len(out) == 0 {
				out = []db.CategoryTag{{Code: "GENERAL.OTHER", Confidence: 0.3}}
			}
			return out
		}
		tags := llm.ClassifyText(s.Title, body)
		out := make([]db.CategoryTag, len(tags))
		for i, t := range tags {
			out[i] = db.CategoryTag{Code: t.Code, Confidence: t.Confidence}
		}
		return out
	}

	var mu sync.Mutex
	byDomain := map[string]int{}
	moved, changed, done := 0, 0, 0
	start := time.Now()

	sem := make(chan struct{}, max(1, concurrency))
	var wg sync.WaitGroup
	for _, s := range sigs {
		wg.Add(1)
		sem <- struct{}{}
		go func(s db.SignalText) {
			defer wg.Done()
			defer func() { <-sem }()
			cats := classify(s)
			code := cats[0].Code

			mu.Lock()
			byDomain[domainPrefix(code)]++
			if code != "GENERAL.OTHER" {
				moved++
			}
			done++
			if done%2000 == 0 {
				fmt.Fprintf(os.Stderr, "  %d/%d...\n", done, len(sigs))
			}
			mu.Unlock()

			if apply && code != "GENERAL.OTHER" {
				if err := d.SetSignalCategory(ctx, s.ID, cats); err != nil {
					fmt.Fprintf(os.Stderr, "update %s: %v\n", s.ID, err)
					return
				}
				mu.Lock()
				changed++
				mu.Unlock()
			}
		}(s)
	}
	wg.Wait()

	fmt.Printf("\nrecategorization %s via %s (%s):\n", ternary(apply, "APPLIED", "DRY-RUN"), mode, time.Since(start).Round(time.Second))
	fmt.Printf("  %d signals scanned, %d moved out of GENERAL (%.1f%%)\n",
		len(sigs), moved, 100*float64(moved)/float64(len(sigs)))
	if apply {
		fmt.Printf("  %d signals updated in place\n", changed)
	}
	printMap("new domain distribution", byDomain)
	if !apply {
		fmt.Println("\n(dry-run — re-run with -apply to persist)")
	}
}

func domainPrefix(code string) string {
	for i := 0; i < len(code); i++ {
		if code[i] == '.' {
			return code[:i]
		}
	}
	return code
}

func ternary(b bool, t, f string) string {
	if b {
		return t
	}
	return f
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
