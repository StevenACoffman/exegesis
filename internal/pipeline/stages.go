package pipeline

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/StevenACoffman/exegesis/internal/book2skill"
	"github.com/StevenACoffman/exegesis/internal/render"
)

// stageConcurrency is the per-stage fan-out limit. The structural Phase-4 gate
// (decision D4) lives in book2skill.ValidateTestSet.
const stageConcurrency = 8

// constructResult pairs a built skill with whether skillcheck was skipped for it.
type constructResult struct {
	skill   book2skill.Skill
	skipped bool
}

// stage0Overview performs analytical reading and writes BOOK_OVERVIEW.md.
func (p *Pipeline) stage0Overview(
	ctx context.Context, bookText string,
) (*book2skill.BookOverview, error) {
	user := "Book title: " + p.Cfg.Title + "\nAuthor: " + p.Cfg.Author +
		"\nYear: " + p.Cfg.Year + "\n\nBook text:\n" + bookText
	o, err := complete[book2skill.BookOverview](ctx, p, overviewSystem, user)
	if err != nil {
		return nil, err
	}
	o.Title, o.Author, o.Year = p.Cfg.Title, p.Cfg.Author, p.Cfg.Year
	if err := p.WriteFile("BOOK_OVERVIEW.md", []byte(render.BookOverview(&o))); err != nil {
		return nil, err
	}
	return &o, nil
}

// stage1Extract runs the five extractors in parallel and returns the merged,
// id-assigned candidate pool, writing per-extractor audit files.
func (p *Pipeline) stage1Extract(
	ctx context.Context, bookText string, o *book2skill.BookOverview,
) ([]book2skill.CandidateUnit, error) {
	exs := extractors()
	overviewJSON := jsonString(o)
	lists, errs := runAll(exs, len(exs),
		func(_ int, ex extractor) ([]book2skill.CandidateUnit, error) {
			return p.runExtractor(ctx, ex, bookText, overviewJSON)
		})
	if err := firstReal(errs); err != nil {
		return nil, err
	}
	if err := firstDeferred(errs); err != nil {
		return nil, err
	}

	var merged []book2skill.CandidateUnit
	for i := range exs {
		if err := p.writeJSON("candidates/"+exs[i].file, lists[i]); err != nil {
			return nil, err
		}
		for j := range lists[i] {
			unit := lists[i][j]
			unit.Type = exs[i].kind
			unit.ID = candidateID(exs[i].kind, len(merged)+1)
			merged = append(merged, unit)
		}
	}
	return merged, nil
}

func (p *Pipeline) runExtractor(
	ctx context.Context, ex extractor, bookText, overviewJSON string,
) ([]book2skill.CandidateUnit, error) {
	type out struct {
		Candidates []book2skill.CandidateUnit `json:"candidates"`
	}
	user := "Quote rune limit: " + strconv.Itoa(p.Cfg.QuoteMaxRunes) +
		"\n\nBook overview (JSON):\n" + overviewJSON + "\n\nBook text:\n" + bookText
	res, err := complete[out](ctx, p, extractSystem+"\n"+ex.guidance, user)
	if err != nil {
		return nil, err
	}
	return res.Candidates, nil
}

// stage15Validate applies triple validation to every candidate in parallel,
// returning the verified units and the count rejected, writing verified.json
// and rejected/*.
func (p *Pipeline) stage15Validate(
	ctx context.Context, candidates []book2skill.CandidateUnit, bookText string,
) ([]book2skill.VerifiedUnit, int, error) {
	vals, errs := runAll(candidates, stageConcurrency,
		func(_ int, c book2skill.CandidateUnit) (book2skill.TripleValidation, error) {
			return p.validateOne(ctx, &c, bookText)
		})
	if err := firstReal(errs); err != nil {
		return nil, 0, err
	}
	if err := firstDeferred(errs); err != nil {
		return nil, 0, err
	}

	var verified []book2skill.VerifiedUnit
	rejected := 0
	for i := range candidates {
		unit := book2skill.VerifiedUnit{Candidate: candidates[i], Validation: vals[i]}
		if vals[i].Passed() {
			verified = append(verified, unit)
			continue
		}
		rejected++
		if err := p.writeJSON("rejected/"+candidates[i].ID+".json", unit); err != nil {
			return nil, 0, err
		}
	}
	if err := p.writeJSON("verified.json", verified); err != nil {
		return nil, 0, err
	}
	return verified, rejected, nil
}

func (p *Pipeline) validateOne(
	ctx context.Context, c *book2skill.CandidateUnit, bookText string,
) (book2skill.TripleValidation, error) {
	user := "Candidate (JSON):\n" + jsonString(c) + "\n\nBook text:\n" + bookText
	return complete[book2skill.TripleValidation](ctx, p, validateSystem, user)
}

// stage2Construct builds a SKILL.md for each verified unit in parallel, runs
// skillcheck, and returns the skills plus whether skillcheck was skipped.
func (p *Pipeline) stage2Construct(
	ctx context.Context, verified []book2skill.VerifiedUnit,
) ([]book2skill.Skill, bool, error) {
	built, errs := runAll(verified, stageConcurrency,
		func(_ int, vu book2skill.VerifiedUnit) (constructResult, error) {
			skill, err := p.constructOne(ctx, &vu)
			if err != nil {
				return constructResult{}, err
			}
			wasSkipped, err := p.writeSkill(ctx, &skill)
			if err != nil {
				return constructResult{}, err
			}
			return constructResult{skill: skill, skipped: wasSkipped}, nil
		})
	if err := firstReal(errs); err != nil {
		return nil, false, err
	}
	if err := firstDeferred(errs); err != nil {
		return nil, false, err
	}

	skills := make([]book2skill.Skill, 0, len(built))
	skipped := false
	for i := range built {
		skills = append(skills, built[i].skill)
		skipped = skipped || built[i].skipped
	}
	return skills, skipped, nil
}

func (p *Pipeline) constructOne(
	ctx context.Context, vu *book2skill.VerifiedUnit,
) (book2skill.Skill, error) {
	user := "Validated unit (JSON):\n" + jsonString(vu)
	return complete[book2skill.Skill](ctx, p, constructSystem, user)
}

func (p *Pipeline) writeSkill(ctx context.Context, s *book2skill.Skill) (bool, error) {
	if err := p.WriteFile(s.Slug+"/SKILL.md", []byte(render.Skill(s))); err != nil {
		return false, err
	}
	if p.Check == nil {
		return false, nil
	}
	skipped, err := p.Check(ctx, s.Slug)
	if err != nil {
		return skipped, err
	}
	return skipped, nil
}

// stage3Link detects relationships between skills, attaches them, and writes
// INDEX.md.
func (p *Pipeline) stage3Link(
	ctx context.Context, o *book2skill.BookOverview, skills []book2skill.Skill,
) error {
	type relOut struct {
		Relationships []book2skill.Relationship `json:"relationships"`
	}
	lines := make([]string, len(skills))
	for i := range skills {
		lines[i] = "- " + skills[i].Slug + ": " + skills[i].Title
	}
	res, err := complete[relOut](ctx, p, relateSystem, "Skills:\n"+strings.Join(lines, "\n"))
	if err != nil {
		return err
	}

	bySlug := make(map[string]*book2skill.Skill, len(skills))
	for i := range skills {
		bySlug[skills[i].Slug] = &skills[i]
	}
	for _, rel := range res.Relationships {
		if skill, ok := bySlug[rel.From]; ok && rel.Kind.Valid() {
			skill.Related = append(skill.Related, rel)
		}
	}
	return p.WriteFile("INDEX.md", []byte(render.Index(o, skills)))
}

// stage4Test generates darwin-compatible test prompts for each skill in parallel.
func (p *Pipeline) stage4Test(ctx context.Context, skills []book2skill.Skill) error {
	errs := runAllErr(skills, stageConcurrency, func(i int, _ book2skill.Skill) error {
		return p.testOne(ctx, &skills[i])
	})
	if err := firstReal(errs); err != nil {
		return err
	}
	if err := firstDeferred(errs); err != nil {
		return err
	}
	return nil
}

func (p *Pipeline) testOne(ctx context.Context, s *book2skill.Skill) error {
	type out struct {
		TestCases []book2skill.TestCase `json:"test_cases"`
	}
	res, err := complete[out](ctx, p, testGenSystem, "Skill (JSON):\n"+jsonString(s))
	if err != nil {
		return err
	}
	if err := validateTestSet(res.TestCases); err != nil {
		return err
	}
	data, err := book2skill.EncodeTestPrompts(res.TestCases)
	if err != nil {
		return &book2skill.Error{Op: "pipeline.testOne", Err: err}
	}
	if err := p.WriteFile(s.Slug+"/test-prompts.json", data); err != nil {
		return err
	}
	return p.WriteFile(s.Slug+"/test-results.md", []byte(testResultsDoc(s.Slug, res.TestCases)))
}

// validateTestSet enforces the structural Phase-4 gate via the domain rule,
// surfacing the first failing reason as a domain error.
func validateTestSet(cases []book2skill.TestCase) error {
	if problems := book2skill.ValidateTestSet(cases); len(problems) > 0 {
		return testSetError(problems[0])
	}
	return nil
}

func testSetError(msg string) error {
	return &book2skill.Error{Code: book2skill.EINVALID, Message: "test set " + msg}
}

func testResultsDoc(slug string, cases []book2skill.TestCase) string {
	counts := book2skill.CountByType(cases)
	return "# Test results — " + slug + "\n\n" +
		"Generated " + strconv.Itoa(len(cases)) + " test cases (" +
		strconv.Itoa(counts[book2skill.ShouldTrigger]) + " should_trigger, " +
		strconv.Itoa(counts[book2skill.ShouldNotTrigger]) + " should_not_trigger, " +
		strconv.Itoa(counts[book2skill.EdgeCase]) + " edge_case).\n\n" +
		"Runtime trigger scoring is delegated to darwin-skill: run\n" +
		"`darwin evolve books/<slug>/` to evaluate and evolve this skill.\n"
}

// runAll applies fn to every item with bounded concurrency, running all items
// to completion (no cancellation) and returning results and per-item errors in
// input order. Running every item — even after one defers — lets a stage record
// its whole batch of deferred prompts in a single pass.
func runAll[T, R any](items []T, limit int, fn func(int, T) (R, error)) ([]R, []error) {
	results := make([]R, len(items))
	errs := make([]error, len(items))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i], errs[i] = fn(i, items[i])
		}()
	}
	wg.Wait()
	return results, errs
}

// runAllErr is runAll for functions that return only an error.
func runAllErr[T any](items []T, limit int, fn func(int, T) error) []error {
	_, errs := runAll(items, limit, func(i int, item T) (struct{}, error) {
		return struct{}{}, fn(i, item)
	})
	return errs
}

// firstReal returns the first non-nil error that is not a deferred-prompt
// sentinel, or nil.
func firstReal(errs []error) error {
	for _, err := range errs {
		if err != nil && !book2skill.IsDeferred(err) {
			return err
		}
	}
	return nil
}

// firstDeferred returns the first deferred-prompt error, or nil.
func firstDeferred(errs []error) error {
	for _, err := range errs {
		if book2skill.IsDeferred(err) {
			return err
		}
	}
	return nil
}
