package appdata

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Mapping is the JSON-defined stub configuration loaded at startup.
// This is intentionally "v1 strict" and small; we can expand it later with richer operators.
type Mapping struct {
	ID          string   `json:"id"`
	Priority    int      `json:"priority,omitempty"` // lower wins; default 1000
	Description string   `json:"description,omitempty"`
	Request     Request  `json:"request"`
	Response    Response `json:"response"`
	Metadata    Metadata `json:"metadata,omitempty"`
}

type Metadata struct {
	Tags    []string `json:"tags,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"` // nil => enabled
}

type Request struct {
	Method     string `json:"method"`
	URLPattern string `json:"urlPattern"`
	// URLMatch controls how urlPattern is matched against the incoming request URL/path.
	// Supported values: "contains" (default), "exact", "prefix", "regex".
	URLMatch string `json:"urlMatch,omitempty"`

	// QueryParams are required query key=value pairs.
	// Example JSON: "queryParams": { "userId": "123", "source": "mobile" }
	// All listed pairs must be present and equal in the incoming request.
	QueryParams map[string]string `json:"queryParams,omitempty"`

	// Body is a set of required JSON field path -> expected value.
	// v1 semantics: all listed fields must exist and equal the expected value.
	// Field paths use dot-notation (e.g. "customer.email").
	Body map[string]any `json:"body,omitempty"`
}

type Response struct {
	Status       int               `json:"status"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         any               `json:"body,omitempty"`
	FixedDelayMs int               `json:"fixedDelayMs,omitempty"`
}

// IncomingRequest is the normalized shape used to match a runtime stub.
// (The HTTP layer can adapt net/http.Request into this.)
type IncomingRequest struct {
	Method string
	// URL is typically the path (e.g. "/users/123/orders"). You can also pass full URL.
	URL   string
	Query map[string][]string
	Body  any
}

// Global is the process-wide runtime index populated on startup.
var Global = NewRuntimeIndex()

type RuntimeIndex struct {
	mu      sync.RWMutex
	methods map[string]*methodNode
	order   int64
	count   int
}

// in-memory snapshot of all loaded mappings (raw config), for admin APIs.
var (
	mappingsMu  sync.RWMutex
	allMappings []Mapping
)

func NewRuntimeIndex() *RuntimeIndex {
	return &RuntimeIndex{
		methods: make(map[string]*methodNode),
	}
}

func (ri *RuntimeIndex) Reset() {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.methods = make(map[string]*methodNode)
	ri.order = 0
	ri.count = 0
}

// ResetMappings clears the global mappings list.
func ResetMappings() {
	mappingsMu.Lock()
	defer mappingsMu.Unlock()
	allMappings = nil
}

// RegisterMapping adds a mapping to the global list for admin/introspection.
func RegisterMapping(m Mapping) {
	mappingsMu.Lock()
	defer mappingsMu.Unlock()
	allMappings = append(allMappings, m)
}

// GetAllMappings returns a copy of all registered mappings.
func GetAllMappings() []Mapping {
	mappingsMu.RLock()
	defer mappingsMu.RUnlock()
	out := make([]Mapping, len(allMappings))
	copy(out, allMappings)
	return out
}

func (ri *RuntimeIndex) Count() int {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return ri.count
}

func (ri *RuntimeIndex) Add(m Mapping) error {
	if strings.TrimSpace(m.ID) == "" {
		return errors.New("missing mapping.id")
	}
	m.Request.Method = strings.ToUpper(strings.TrimSpace(m.Request.Method))
	if m.Request.Method == "" {
		return fmt.Errorf("mapping %q: missing request.method", m.ID)
	}
	if strings.TrimSpace(m.Request.URLPattern) == "" {
		return fmt.Errorf("mapping %q: missing request.urlPattern", m.ID)
	}
	if m.Priority == 0 {
		m.Priority = 1000
	}
	enabled := true
	if m.Metadata.Enabled != nil {
		enabled = *m.Metadata.Enabled
	}
	if !enabled {
		// keep it out of the runtime index entirely; we can change this later if you want hot toggles.
		return nil
	}

	ri.mu.Lock()
	ri.order++
	order := ri.order
	ri.mu.Unlock()

	cs, err := compileStub(m, order)
	if err != nil {
		return err
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	mn := ri.methods[m.Request.Method]
	if mn == nil {
		mn = &methodNode{}
		ri.methods[m.Request.Method] = mn
	}
	mn.addCompiled(cs)
	ri.count++
	return nil
}

// FindBestMatch matches a request to the best stub based on:
// priority asc (lower wins) -> specificity score desc -> load order asc.
func (ri *RuntimeIndex) FindBestMatch(req IncomingRequest) (Mapping, bool) {
	method := strings.ToUpper(strings.TrimSpace(req.Method))

	ri.mu.RLock()
	mn := ri.methods[method]
	ri.mu.RUnlock()
	if mn == nil {
		return Mapping{}, false
	}

	best, ok := mn.findBest(req)
	if !ok {
		return Mapping{}, false
	}
	return best.mapping, true
}

// ---- internal tree nodes ----

type methodNode struct {
	// "Tree" levels: URL -> Query requirements -> Body requirements -> stubs
	urls []*urlNode
}

func (mn *methodNode) addCompiled(cs *compiledStub) {
	// URL level
	un := mn.findOrCreateURLNode(cs)
	// Query level
	qn := un.findOrCreateQueryNode(cs.querySignature, cs.queryPairs)
	// Body level
	bn := qn.findOrCreateBodyNode(cs.bodySignature, cs.bodyMatchers)
	bn.stubs = append(bn.stubs, cs)
}

func (mn *methodNode) findOrCreateURLNode(cs *compiledStub) *urlNode {
	key := cs.urlKey()
	for _, n := range mn.urls {
		if n.key == key {
			return n
		}
	}
	n := &urlNode{
		key:     key,
		regex:   cs.regex,
		queries: make(map[string]*queryNode),
	}
	mn.urls = append(mn.urls, n)
	return n
}

func (mn *methodNode) findBest(req IncomingRequest) (*compiledStub, bool) {
	var best *compiledStub
	bestPriority := int(^uint(0) >> 1) // max int
	bestScore := -1
	var bestOrder int64 = 1<<63 - 1

	for _, un := range mn.urls {
		if !un.matchesURL(req.URL) {
			continue
		}
		for _, qn := range un.queries {
			if !qn.matchesQuery(req.Query) {
				continue
			}
			for _, bn := range qn.bodies {
				if !bn.matchesBody(req.Body) {
					continue
				}
				for _, cs := range bn.stubs {
					score := cs.specificityScore()
					p := cs.mapping.Priority
					if p < bestPriority ||
						(p == bestPriority && score > bestScore) ||
						(p == bestPriority && score == bestScore && cs.order < bestOrder) {
						best = cs
						bestPriority = p
						bestScore = score
						bestOrder = cs.order
					}
				}
			}
		}
	}

	return best, best != nil
}

type urlKey struct {
	kind    urlMatchKind
	pattern string
}

type urlNode struct {
	key   urlKey
	regex *regexp.Regexp
	// querySignature -> queryNode
	queries map[string]*queryNode
}

func (un *urlNode) matchesURL(u string) bool {
	if un.key.kind == urlMatchRegex && un.regex != nil {
		return un.regex.MatchString(u)
	}
	return un.key.kind.match(un.key.pattern, u)
}

func (un *urlNode) findOrCreateQueryNode(sig string, pairs []queryPair) *queryNode {
	if existing := un.queries[sig]; existing != nil {
		return existing
	}
	n := &queryNode{
		signature: sig,
		required:  pairs,
		bodies:    make(map[string]*bodyNode),
	}
	un.queries[sig] = n
	return n
}

type queryNode struct {
	signature string
	required  []queryPair // sorted
	bodies    map[string]*bodyNode
}

func (qn *queryNode) matchesQuery(query map[string][]string) bool {
	if len(qn.required) == 0 {
		return true
	}
	for _, p := range qn.required {
		values, ok := query[p.Key]
		if !ok {
			return false
		}
		if !containsString(values, p.Value) {
			return false
		}
	}
	return true
}

func (qn *queryNode) findOrCreateBodyNode(sig string, matchers []bodyFieldMatcher) *bodyNode {
	if existing := qn.bodies[sig]; existing != nil {
		return existing
	}
	n := &bodyNode{
		signature: sig,
		matchers:  matchers,
	}
	qn.bodies[sig] = n
	return n
}

type bodyNode struct {
	signature string
	matchers  []bodyFieldMatcher
	stubs     []*compiledStub
}

func (bn *bodyNode) matchesBody(body any) bool {
	if len(bn.matchers) == 0 {
		return true
	}
	for _, m := range bn.matchers {
		if !m.match(body) {
			return false
		}
	}
	return true
}

// ---- compilation + match helpers ----

type urlMatchKind int

const (
	urlMatchContains urlMatchKind = iota
	urlMatchExact
	urlMatchPrefix
	urlMatchRegex
)

func parseURLMatchKind(s string) urlMatchKind {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "contains":
		return urlMatchContains
	case "exact":
		return urlMatchExact
	case "prefix":
		return urlMatchPrefix
	case "regex":
		return urlMatchRegex
	default:
		// be strict by default: treat unknown as contains
		return urlMatchContains
	}
}

func (k urlMatchKind) match(pattern, u string) bool {
	switch k {
	case urlMatchExact:
		return u == pattern
	case urlMatchPrefix:
		return strings.HasPrefix(u, pattern)
	case urlMatchRegex:
		// Regex matching is handled by urlNode (which holds the compiled regex).
		// Fallback to a safe contains match if called unexpectedly.
		return strings.Contains(u, pattern)
	case urlMatchContains:
		fallthrough
	default:
		return strings.Contains(u, pattern)
	}
}

type bodyFieldMatcher struct {
	path     string
	expected any
}

func (m bodyFieldMatcher) match(body any) bool {
	v, ok := lookupDotPath(body, m.path)
	if !ok {
		return false
	}
	return valuesEqual(v, m.expected)
}

func lookupDotPath(body any, path string) (any, bool) {
	cur := body
	for _, part := range strings.Split(path, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := obj[part]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func valuesEqual(actual, expected any) bool {
	// Handle common JSON decode shapes: numbers become float64.
	switch e := expected.(type) {
	case float64:
		switch a := actual.(type) {
		case float64:
			return a == e
		case int:
			return float64(a) == e
		case int64:
			return float64(a) == e
		}
	case string:
		if a, ok := actual.(string); ok {
			return a == e
		}
	case bool:
		if a, ok := actual.(bool); ok {
			return a == e
		}
	}
	// Fallback: stringified equality (simple v1 behavior)
	return fmt.Sprint(actual) == fmt.Sprint(expected)
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

type compiledStub struct {
	mapping Mapping
	order   int64

	urlKind urlMatchKind
	pattern string
	regex   *regexp.Regexp

	queryPairs     []queryPair
	querySignature string

	bodyMatchers  []bodyFieldMatcher
	bodySignature string
}

type queryPair struct {
	Key   string
	Value string
}

func (cs *compiledStub) urlKey() urlKey {
	return urlKey{kind: cs.urlKind, pattern: cs.pattern}
}

func (cs *compiledStub) specificityScore() int {
	score := 0
	switch cs.urlKind {
	case urlMatchExact:
		score += 1000
	case urlMatchPrefix:
		score += 800
	case urlMatchContains:
		score += 600
	case urlMatchRegex:
		score += 400
	}
	// More literal characters usually means more specific.
	score += min(len(cs.pattern), 200)
	score += 10 * len(cs.queryPairs)
	score += 20 * len(cs.bodyMatchers)
	return score
}

func compileStub(m Mapping, order int64) (*compiledStub, error) {
	kind := parseURLMatchKind(m.Request.URLMatch)
	cs := &compiledStub{
		mapping:       m,
		order:         order,
		urlKind:       kind,
		pattern:       m.Request.URLPattern,
		bodySignature: "",
	}

	if kind == urlMatchRegex {
		re, err := regexp.Compile(m.Request.URLPattern)
		if err != nil {
			return nil, fmt.Errorf("mapping %q: invalid urlPattern regex: %w", m.ID, err)
		}
		cs.regex = re
	}

	// Query signature: sort key=value pairs for determinism.
	if len(m.Request.QueryParams) > 0 {
		pairs := make([]queryPair, 0, len(m.Request.QueryParams))
		for k, v := range m.Request.QueryParams {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			if k == "" {
				continue
			}
			pairs = append(pairs, queryPair{Key: k, Value: v})
		}
		sort.Slice(pairs, func(i, j int) bool {
			if pairs[i].Key == pairs[j].Key {
				return pairs[i].Value < pairs[j].Value
			}
			return pairs[i].Key < pairs[j].Key
		})
		cs.queryPairs = pairs

		sigParts := make([]string, 0, len(pairs))
		for _, p := range pairs {
			sigParts = append(sigParts, fmt.Sprintf("%s=%s", p.Key, p.Value))
		}
		cs.querySignature = strings.Join(sigParts, "&")
	} else {
		cs.queryPairs = nil
		cs.querySignature = ""
	}

	// Body matchers
	if len(m.Request.Body) > 0 {
		paths := make([]string, 0, len(m.Request.Body))
		for p := range m.Request.Body {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		var sigParts []string
		for _, p := range paths {
			exp := m.Request.Body[p]
			cs.bodyMatchers = append(cs.bodyMatchers, bodyFieldMatcher{path: p, expected: exp})
			sigParts = append(sigParts, fmt.Sprintf("%s=%s", p, fmt.Sprint(exp)))
		}
		cs.bodySignature = strings.Join(sigParts, "|")
	} else {
		cs.bodySignature = ""
	}

	return cs, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
