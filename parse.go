package grammar

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parse builds a Grammar from a source-format string. Errors carry the
// 1-based line and column where the problem starts.
func Parse(source string) (*Grammar, error) {
	p := &parser{
		grammar: &Grammar{rules: map[string]*Rule{}},
		lines:   splitLines(source),
	}
	return p.run()
}

// parser walks the source line-by-line. The grammar is line-oriented:
// rule headers, the forms declaration, and entries are each one line.
type parser struct {
	grammar *Grammar
	lines   []sourceLine

	// currentRule, if non-nil, is the rule still accepting entries.
	// The forms-declaration line for that rule has already been
	// consumed; subsequent non-blank lines are entries until a blank
	// line or the next "rule" keyword.
	currentRule     *Rule
	currentRuleName string
	currentRuleLine int
	formsDeclared   bool
}

type sourceLine struct {
	num  int    // 1-based
	text string // raw line, no trailing \n
}

func splitLines(src string) []sourceLine {
	if src == "" {
		return nil
	}
	parts := strings.Split(src, "\n")
	// A trailing '\n' produces an empty final element; drop it so
	// callers don't see a phantom blank line at end-of-file.
	if n := len(parts); n > 0 && parts[n-1] == "" {
		parts = parts[:n-1]
	}
	out := make([]sourceLine, 0, len(parts))
	for i, p := range parts {
		// Strip a trailing '\r' so CRLF-terminated files parse the
		// same as LF-terminated ones.
		p = strings.TrimSuffix(p, "\r")
		out = append(out, sourceLine{num: i + 1, text: p})
	}
	return out
}

func (p *parser) run() (*Grammar, error) {
	for _, ln := range p.lines {
		// A comment-only line is whitespace from the parser's point of
		// view, but it does not separate rules — only a truly blank
		// source line does. Track both so the rule-block boundary is
		// "no non-comment text on the line after stripping comments
		// AND no text at all in the original line."
		stripped, _ := stripComment(ln.text)
		trimmed := strings.TrimSpace(stripped)
		rawTrimmed := strings.TrimSpace(ln.text)
		if trimmed == "" {
			if rawTrimmed == "" {
				if err := p.endRule(); err != nil {
					return nil, err
				}
			}
			continue
		}
		if isRuleHeader(trimmed) {
			if err := p.startRule(ln, trimmed); err != nil {
				return nil, err
			}
			continue
		}
		if p.currentRule == nil {
			return nil, parseErrorf(ln.num, indexOf(ln.text, trimmed)+1, "expected blank line or rule header, got %q", trimmed)
		}
		// Pass the comment-stripped line content downstream. Column
		// reporting is based on offsets inside `stripped`, which has the
		// same column layout as the original line up to the comment.
		ln2 := sourceLine{num: ln.num, text: stripped}
		if !p.formsDeclared {
			if strings.HasPrefix(trimmed, "forms:") {
				if err := p.parseFormsLine(ln2, trimmed); err != nil {
					return nil, err
				}
				p.formsDeclared = true
				continue
			}
			// Implicit single-default-form rule: the first non-blank
			// line is an entry, so synthesise a `forms: default` for
			// the rule and fall through to the entry-line handler.
			p.currentRule.Forms = []FormSpec{{Name: "default"}}
			p.formsDeclared = true
		} else if strings.HasPrefix(trimmed, "forms:") {
			return nil, parseErrorf(ln.num, indexOf(ln.text, trimmed)+1, "rule %q has multiple forms: lines or forms: after an entry", p.currentRuleName)
		}
		if err := p.parseEntryLine(ln2, trimmed); err != nil {
			return nil, err
		}
	}
	if err := p.endRule(); err != nil {
		return nil, err
	}
	return p.grammar, nil
}

func isRuleHeader(s string) bool {
	if s == "rule" {
		return true
	}
	return strings.HasPrefix(s, "rule ") || strings.HasPrefix(s, "rule\t")
}

func (p *parser) endRule() error {
	if p.currentRule == nil {
		return nil
	}
	if len(p.currentRule.Alternatives) == 0 {
		return fmt.Errorf("rule %q at line %d has no entries", p.currentRuleName, p.currentRuleLine)
	}
	p.grammar.rules[p.currentRuleName] = p.currentRule
	p.currentRule = nil
	p.currentRuleName = ""
	p.formsDeclared = false
	return nil
}

func (p *parser) startRule(ln sourceLine, trimmed string) error {
	if err := p.endRule(); err != nil {
		return err
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "rule"))
	if rest == "" {
		return parseErrorf(ln.num, indexOf(ln.text, "rule")+1, "rule keyword without a name")
	}
	if !isRuleName(rest) {
		return parseErrorf(ln.num, indexOf(ln.text, rest)+1, "invalid rule name %q (must match [a-z][a-z0-9_]*)", rest)
	}
	if _, exists := p.grammar.rules[rest]; exists {
		return parseErrorf(ln.num, indexOf(ln.text, rest)+1, "%w: %q", ErrDuplicateRule, rest)
	}
	p.currentRule = &Rule{}
	p.currentRuleName = rest
	p.currentRuleLine = ln.num
	p.formsDeclared = false
	return nil
}

// parseFormsLine consumes a "forms: NAME[=TPL] {, NAME[=TPL]}" line and
// populates p.currentRule.Forms.
func (p *parser) parseFormsLine(ln sourceLine, trimmed string) error {
	// Locate the body inside ln.text so we can report 1-based columns
	// that point into the original line, not the trimmed copy.
	formsIdx := indexOf(ln.text, "forms:")
	bodyStart := formsIdx + len("forms:")
	for bodyStart < len(ln.text) && (ln.text[bodyStart] == ' ' || ln.text[bodyStart] == '\t') {
		bodyStart++
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "forms:"))
	if body == "" {
		return parseErrorf(ln.num, formsIdx+1, "forms: declaration is empty")
	}
	// Walk comma-separated parts of body while tracking the offset of
	// each part inside ln.text, so column reporting points at the
	// problematic occurrence rather than the first occurrence of the
	// same substring elsewhere on the line.
	cursor := bodyStart
	first := true
	seen := map[string]bool{}
	for {
		// Find the next comma in ln.text starting at cursor.
		end := cursor
		for end < len(ln.text) && ln.text[end] != ',' {
			end++
		}
		raw := ln.text[cursor:end]
		// 1-based column of the first non-space byte in raw, or of the
		// raw itself if it's empty.
		entryStart := cursor
		for entryStart < end && (ln.text[entryStart] == ' ' || ln.text[entryStart] == '\t') {
			entryStart++
		}
		entry := strings.TrimSpace(raw)
		col := entryStart + 1
		var name string
		var defaultExpr string
		var defaultStart int
		var hasDefault bool
		if eq := strings.IndexByte(entry, '='); eq >= 0 {
			name = strings.TrimSpace(entry[:eq])
			defaultExpr = entry[eq+1:]
			// The defaultExpr begins at entryStart + (eq+1) inside
			// ln.text. Trim leading whitespace from the template body
			// per spec; trailing whitespace inside the template is
			// preserved.
			defaultStart = entryStart + eq + 1
			for len(defaultExpr) > 0 && (defaultExpr[0] == ' ' || defaultExpr[0] == '\t') {
				defaultExpr = defaultExpr[1:]
				defaultStart++
			}
			hasDefault = true
		} else {
			name = entry
		}
		if !isFormName(name) {
			return parseErrorf(ln.num, col, "invalid form name %q (must match [a-z][a-z0-9_]*)", name)
		}
		if seen[name] {
			return parseErrorf(ln.num, col, "form %q declared twice", name)
		}
		seen[name] = true
		spec := FormSpec{Name: name}
		if hasDefault {
			if first {
				return parseErrorf(ln.num, col, "default form %q cannot have a default template", name)
			}
			tpl, err := parseTemplate(defaultExpr, ln.num, defaultStart+1, true)
			if err != nil {
				return err
			}
			spec.Default = tpl
		}
		p.currentRule.Forms = append(p.currentRule.Forms, spec)
		first = false
		if end >= len(ln.text) || ln.text[end] != ',' {
			break
		}
		cursor = end + 1
	}
	if len(p.currentRule.Forms) == 0 {
		return parseErrorf(ln.num, formsIdx+1, "forms: declaration is empty")
	}
	return nil
}

func (p *parser) parseEntryLine(ln sourceLine, trimmed string) error {
	// Compute the offset of the trimmed entry inside ln.text so column
	// reporting on each form value points at the right occurrence even
	// when the same substring appears more than once on the line.
	bodyStart := indexOf(ln.text, trimmed)
	body := trimmed
	weight := uint(1)
	if strings.HasPrefix(body, "weight=") {
		// Take everything up to the first whitespace as the weight tag.
		end := strings.IndexAny(body, " \t")
		if end < 0 {
			return parseErrorf(ln.num, bodyStart+1, "weight= tag without an entry body")
		}
		w, err := parseWeight(body[len("weight="):end])
		if err != nil {
			return parseErrorf(ln.num, bodyStart+1, "weight: %v", err)
		}
		weight = w
		// Skip the weight= prefix and any whitespace after it.
		newBody := strings.TrimLeft(body[end:], " \t")
		bodyStart += len(body) - len(newBody)
		body = newBody
	}
	var tags []string
	var err error
	body, tags, err = parseTrailingTags(body, ln.num, bodyStart+1)
	if err != nil {
		return err
	}
	// Split body on top-level pipes (pipes inside {...} or part of a
	// recognised backslash escape don't split). Track each value's
	// starting offset inside ln.text for column reporting.
	values, offsets := splitTopLevelPipesWithOffsets(body)
	if len(values) > len(p.currentRule.Forms) {
		return parseErrorf(ln.num, bodyStart+1, "rule %q has %d forms but entry supplies %d", p.currentRuleName, len(p.currentRule.Forms), len(values))
	}
	alt := Alternative{Weight: weight, Forms: map[string]Template{}, Tags: tags}
	for i, v := range values {
		// Trim surrounding whitespace; track the new column so error
		// messages point at the trimmed start, not the leading space.
		origOff := offsets[i]
		left := 0
		for left < len(v) && (v[left] == ' ' || v[left] == '\t') {
			left++
		}
		trimmedV := strings.TrimSpace(v)
		formName := p.currentRule.Forms[i].Name
		if trimmedV == "" {
			if i == 0 {
				return parseErrorf(ln.num, bodyStart+origOff+1, "rule %q entry must supply the default form", p.currentRuleName)
			}
			// An empty value is only legal as a trailing omission;
			// rejecting middle omissions matches the spec ("Middle-form
			// omission is not supported in v1"). Detect by looking
			// ahead for any non-empty later value.
			for _, lv := range values[i+1:] {
				if strings.TrimSpace(lv) != "" {
					return parseErrorf(ln.num, bodyStart+origOff+1, "rule %q: middle-form omission is not supported (only trailing forms may be omitted)", p.currentRuleName)
				}
			}
			continue
		}
		tpl, err := parseTemplate(trimmedV, ln.num, bodyStart+origOff+left+1, false)
		if err != nil {
			return err
		}
		alt.Forms[formName] = tpl
	}
	if _, hasDefault := alt.Forms[p.currentRule.Forms[0].Name]; !hasDefault {
		return parseErrorf(ln.num, bodyStart+1, "rule %q entry must supply the default form", p.currentRuleName)
	}
	p.currentRule.Alternatives = append(p.currentRule.Alternatives, alt)
	return nil
}

func parseTrailingTags(body string, line, col int) (string, []string, error) {
	trimmedRight := strings.TrimRight(body, " \t")
	tagStart := trailingTagStart(trimmedRight)
	if tagStart < 0 {
		return body, nil, nil
	}
	raw := trimmedRight[tagStart+len("tags="):]
	if raw == "" {
		return "", nil, parseErrorf(line, col+tagStart, "tags= declaration is empty")
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if !isTagName(part) {
			return "", nil, parseErrorf(line, col+tagStart, "invalid tag %q (%s)", part, invalidTagDescription)
		}
		tags = append(tags, part)
	}
	body = strings.TrimRight(trimmedRight[:tagStart], " \t")
	return body, tags, nil
}

func trailingTagStart(s string) int {
	const prefix = "tags="
	if !strings.HasPrefix(s, prefix) && !strings.Contains(s, " "+prefix) && !strings.Contains(s, "\t"+prefix) {
		return -1
	}
	depth := 0
	last := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if c == '{' {
			depth++
			continue
		}
		if c == '}' && depth > 0 {
			depth--
			continue
		}
		if depth == 0 && strings.HasPrefix(s[i:], prefix) && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			last = i
		}
	}
	if last < 0 {
		return -1
	}
	return last
}

// splitTopLevelPipesWithOffsets splits s on top-level '|' and returns
// each chunk with its starting byte offset in s. Pipes inside {...} or
// preceded by a backslash do not split — the backslash protects the
// next byte from being interpreted as a separator. The chunk itself
// keeps both bytes; whether the escape is *decoded* (e.g. \\ -> \) is
// the template parser's job.
func splitTopLevelPipesWithOffsets(s string) ([]string, []int) {
	var values []string
	var offsets []int
	var sb strings.Builder
	chunkStart := 0
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			sb.WriteByte(c)
			sb.WriteByte(s[i+1])
			i++
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' && depth > 0 {
			depth--
		}
		if c == '|' && depth == 0 {
			values = append(values, sb.String())
			offsets = append(offsets, chunkStart)
			sb.Reset()
			chunkStart = i + 1
			continue
		}
		sb.WriteByte(c)
	}
	values = append(values, sb.String())
	offsets = append(offsets, chunkStart)
	return values, offsets
}

func parseWeight(s string) (uint, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("weight %q is not an integer", s)
	}
	if n < 1 {
		return 0, fmt.Errorf("weight must be >= 1, got %d", n)
	}
	return uint(n), nil
}

// parseTemplate turns a raw template string into a Template. inFormDefault
// allows the empty-braces SelfRef; entry templates pass false and any {}
// in them is an error.
func parseTemplate(s string, line, col int, inFormDefault bool) (Template, error) {
	tpl := Template{}
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			tpl = append(tpl, Literal{Text: lit.String()})
			lit.Reset()
		}
	}
	i := 0
	for i < len(s) {
		c := s[i]
		switch c {
		case '\\':
			if i+1 < len(s) {
				next := s[i+1]
				if next == '{' || next == '}' || next == '#' || next == '\\' {
					lit.WriteByte(next)
					i += 2
					continue
				}
			}
			// Any other "\X" is a literal backslash; X re-evaluates on
			// the next iteration. In particular, '\|' is two literals.
			lit.WriteByte('\\')
			i++
		case '{':
			flush()
			end := strings.IndexByte(s[i+1:], '}')
			if end < 0 {
				return nil, parseErrorf(line, col+i, "unclosed {")
			}
			body := s[i+1 : i+1+end]
			tok, err := parseRefBody(body, line, col+i, inFormDefault)
			if err != nil {
				return nil, err
			}
			tpl = append(tpl, tok)
			i += end + 2
		default:
			lit.WriteByte(c)
			i++
		}
	}
	flush()
	return tpl, nil
}

// parseRefBody parses the contents between { and } and returns the
// matching Token. It handles SelfRef ({}), Recall ({*NAME}), and
// RuleRef ({rule[:form][|tags=...][|required=...][ as NAME]}).
func parseRefBody(body string, line, col int, inFormDefault bool) (Token, error) {
	if body == "" {
		if !inFormDefault {
			return nil, parseErrorf(line, col, "empty {} is only valid inside a non-default form's default template")
		}
		return SelfRef{}, nil
	}
	if body[0] == '*' {
		name := body[1:]
		if !isSaveName(name) {
			return nil, parseErrorf(line, col, "invalid recall name %q (must match [A-Z][A-Z0-9_]*)", name)
		}
		return Recall{Name: name}, nil
	}
	// Rule-ref: NAME [":" FORM] [ws "as" ws SAVE]
	rest := body
	var save string
	if idx := findAsKeyword(rest); idx >= 0 {
		save = strings.TrimSpace(rest[idx+len("as"):])
		// Trim the trailing whitespace before "as" too.
		rest = strings.TrimRight(rest[:idx], " \t")
		if !isSaveName(save) {
			return nil, parseErrorf(line, col, "invalid save name %q (must match [A-Z][A-Z0-9_]*)", save)
		}
	}
	base, options, hasOptions := strings.Cut(rest, "|")
	if hasOptions && options == "" {
		return nil, parseErrorf(line, col+len(base)+1, "empty rule reference option")
	}
	tags, required, err := parseRefOptions(options, line, col+len(base)+1)
	if err != nil {
		return nil, err
	}
	rest = base
	ruleName, formName, hasForm := strings.Cut(rest, ":")
	if hasForm && !isFormName(formName) {
		return nil, parseErrorf(line, col, "invalid form name %q in rule reference", formName)
	}
	if !isRuleName(ruleName) {
		return nil, parseErrorf(line, col, "invalid rule name %q in reference", ruleName)
	}
	return RuleRef{Rule: ruleName, Form: formName, Save: save, Tags: tags, Required: required}, nil
}

func parseRefOptions(options string, line, col int) ([]string, []string, error) {
	if options == "" {
		return nil, nil, nil
	}
	var tags []string
	var required []string
	cursor := col
	for option := range strings.SplitSeq(options, "|") {
		if option == "" {
			return nil, nil, parseErrorf(line, cursor, "empty rule reference option")
		}
		name, raw, ok := strings.Cut(option, "=")
		if !ok {
			return nil, nil, parseErrorf(line, cursor, "rule reference option %q needs =", option)
		}
		switch name {
		case "tags":
			parsed, err := parseRefTagList(raw, line, cursor+len(name)+1, true)
			if err != nil {
				return nil, nil, err
			}
			tags = append(tags, parsed...)
		case "required":
			parsed, err := parseRefTagList(raw, line, cursor+len(name)+1, false)
			if err != nil {
				return nil, nil, err
			}
			required = append(required, parsed...)
		default:
			return nil, nil, parseErrorf(line, cursor, "unknown rule reference option %q", name)
		}
		cursor += len(option) + 1
	}
	return tags, required, nil
}

func parseRefTagList(raw string, line, col int, allowRemoval bool) ([]string, error) {
	if raw == "" {
		return nil, parseErrorf(line, col, "tag list is empty")
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if err := validateReferenceTags([]string{part}, allowRemoval); err != nil {
			return nil, parseErrorf(line, col, "invalid tag %q (%s)", part, invalidTagDescription)
		}
		tags = append(tags, part)
	}
	return tags, nil
}

// findAsKeyword finds " as " (or "\tas\t", etc) in a ref body. The spec
// requires at least one whitespace either side, so a literal "as" inside
// a name doesn't match.
func findAsKeyword(s string) int {
	for i := 1; i+3 <= len(s); i++ {
		if (s[i-1] == ' ' || s[i-1] == '\t') && s[i] == 'a' && s[i+1] == 's' &&
			(i+2 == len(s) || s[i+2] == ' ' || s[i+2] == '\t') {
			return i
		}
	}
	return -1
}

func isRuleName(s string) bool {
	if s == "" {
		return false
	}
	if !(s[0] >= 'a' && s[0] <= 'z') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

// isFormName matches the same character class as isRuleName; the spec
// gives them the same regex.
func isFormName(s string) bool { return isRuleName(s) }

func isSaveName(s string) bool {
	if s == "" {
		return false
	}
	if !(s[0] >= 'A' && s[0] <= 'Z') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

// stripComment trims the part of line at and after an unescaped '#'
// that lies outside any {...}. Returns the cleaned line and whether a
// comment was present.
func stripComment(s string) (string, bool) {
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Any '\X' advances two chars. Comment-stripping happens before
		// template parsing, so we don't care which X it is — even '\#'
		// just protects the '#' from being seen as a comment start.
		if c == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' && depth > 0 {
			depth--
		}
		if c == '#' && depth == 0 {
			return s[:i], true
		}
	}
	return s, false
}

// indexOf returns the byte offset of needle in haystack, or 0 if not
// present. The returned value is intended for column reporting; we
// add 1 at the call site to get a 1-based column.
func indexOf(haystack, needle string) int {
	if needle == "" {
		return 0
	}
	idx := strings.Index(haystack, needle)
	if idx < 0 {
		return 0
	}
	return idx
}

// ParseError is the error returned by Parse for source-format problems.
// It always carries a 1-based line and column so callers can highlight
// the offending input. When the message was built with a %w verb the
// wrapped error is also accessible via errors.Is / errors.Unwrap.
type ParseError struct {
	Line    int
	Col     int
	Msg     string
	wrapped error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("grammar parse error at %d:%d: %s", e.Line, e.Col, e.Msg)
}

func (e *ParseError) Unwrap() error { return e.wrapped }

func parseErrorf(line, col int, format string, args ...any) *ParseError {
	err := fmt.Errorf(format, args...)
	return &ParseError{Line: line, Col: col, Msg: err.Error(), wrapped: errors.Unwrap(err)}
}
