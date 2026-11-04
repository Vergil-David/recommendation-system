package embeddings

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// DefaultDimension is the default embedding vector size.
// 128 dimensions is a good balance between quality and performance for pgvector.
const DefaultDimension = 384

// ItemText holds the textual data used to generate an embedding.
type ItemText struct {
	Title       string
	Description string
	Genres      []string
}

// Generator produces fixed-dimension embeddings from item text using
// TF-IDF weighted feature hashing (the "hashing trick").
//
// Algorithm overview:
//  1. Fit phase: scan all items to compute document frequencies (IDF).
//  2. Transform phase: for each item, tokenize text fields, compute
//     TF-IDF weights, hash tokens into a fixed-size vector, and L2-normalize.
//
// Weighted fields:
//   - Title tokens:       weight 3.0 (unigrams) + 2.0 (bigrams)
//   - Genre tokens:       weight 2.5
//   - Description tokens: weight 1.0 (unigrams) + 0.5 (bigrams)
type Generator struct {
	dimension int
	docFreq   map[string]int
	totalDocs int
	fitted    bool
}

// NewGenerator creates a new embedding generator with the given vector dimension.
func NewGenerator(dimension int) *Generator {
	if dimension <= 0 {
		dimension = DefaultDimension
	}
	return &Generator{
		dimension: dimension,
		docFreq:   make(map[string]int),
	}
}

// Fit computes inverse document frequencies from the entire corpus.
// Must be called before Transform.
func (g *Generator) Fit(items []ItemText) {
	g.totalDocs = len(items)
	g.docFreq = make(map[string]int, 1024)

	for _, item := range items {
		seen := make(map[string]struct{}, 64)
		tokens := g.extractAllTokens(item)
		for _, token := range tokens {
			if _, exists := seen[token]; !exists {
				seen[token] = struct{}{}
				g.docFreq[token]++
			}
		}
	}
	g.fitted = true
}

// Transform generates an embedding vector for a single item.
// Fit must be called first to compute IDF weights.
func (g *Generator) Transform(item ItemText) []float32 {
	vector := make([]float64, g.dimension)

	titleTokens := tokenize(item.Title)
	g.addWeightedTokens(vector, titleTokens, 3.0)
	g.addWeightedBigrams(vector, titleTokens, 2.0)

	genreTokens := make([]string, 0, len(item.Genres)*2)
	for _, genre := range item.Genres {
		tokens := tokenize(genre)
		genreTokens = append(genreTokens, tokens...)
		normalized := strings.ToLower(strings.TrimSpace(genre))
		if normalized != "" {
			genreTokens = append(genreTokens, "genre:"+normalized)
		}
	}
	g.addWeightedTokens(vector, genreTokens, 2.5)

	descTokens := tokenize(item.Description)
	g.addWeightedTokens(vector, descTokens, 1.0)
	g.addWeightedBigrams(vector, descTokens, 0.5)

	l2Normalize(vector)

	result := make([]float32, g.dimension)
	for i, v := range vector {
		result[i] = float32(v)
	}
	return result
}

// extractAllTokens returns all unique tokens (unigrams + bigrams) for IDF counting.
func (g *Generator) extractAllTokens(item ItemText) []string {
	var all []string

	titleTokens := tokenize(item.Title)
	all = append(all, titleTokens...)
	all = append(all, bigrams(titleTokens)...)

	for _, genre := range item.Genres {
		tokens := tokenize(genre)
		all = append(all, tokens...)
		normalized := strings.ToLower(strings.TrimSpace(genre))
		if normalized != "" {
			all = append(all, "genre:"+normalized)
		}
	}

	descTokens := tokenize(item.Description)
	all = append(all, descTokens...)
	all = append(all, bigrams(descTokens)...)

	return all
}

// idf computes the smoothed inverse document frequency for a token.
// Formula: log((N + 1) / (df + 1)) + 1.0
func (g *Generator) idf(token string) float64 {
	if !g.fitted || g.totalDocs == 0 {
		return 1.0
	}
	df := g.docFreq[token]
	if df == 0 {
		return 1.0
	}
	return math.Log(float64(g.totalDocs+1)/float64(df+1)) + 1.0
}

// addWeightedTokens hashes tokens into the vector with TF-IDF × field weight.
func (g *Generator) addWeightedTokens(vector []float64, tokens []string, fieldWeight float64) {
	tf := termFrequencies(tokens)
	for token, count := range tf {
		bucket := hashBucket(token, g.dimension)
		sign := hashSign(token)
		weight := float64(count) * g.idf(token) * fieldWeight
		vector[bucket] += sign * weight
	}
}

// addWeightedBigrams hashes bigrams into the vector with TF-IDF × field weight.
func (g *Generator) addWeightedBigrams(vector []float64, tokens []string, fieldWeight float64) {
	bgs := bigrams(tokens)
	tf := termFrequencies(bgs)
	for token, count := range tf {
		bucket := hashBucket(token, g.dimension)
		sign := hashSign(token)
		weight := float64(count) * g.idf(token) * fieldWeight
		vector[bucket] += sign * weight
	}
}

// hashBucket maps a token to a vector index using FNV-1a.
func hashBucket(token string, dimension int) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	return int(h.Sum32()) % dimension
}

// hashSign returns +1 or -1 for a token (reduces collision effects).
func hashSign(token string) float64 {
	h := fnv.New32()
	_, _ = h.Write([]byte(token))
	if h.Sum32()%2 == 0 {
		return 1.0
	}
	return -1.0
}

// l2Normalize scales the vector to unit length.
func l2Normalize(vector []float64) {
	var norm float64
	for _, v := range vector {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm < 1e-12 {
		return
	}
	for i := range vector {
		vector[i] /= norm
	}
}

// termFrequencies counts occurrences of each token.
func termFrequencies(tokens []string) map[string]int {
	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	return tf
}

// bigrams produces consecutive token pairs.
func bigrams(tokens []string) []string {
	if len(tokens) < 2 {
		return nil
	}
	result := make([]string, 0, len(tokens)-1)
	for i := 0; i < len(tokens)-1; i++ {
		result = append(result, tokens[i]+"_"+tokens[i+1])
	}
	return result
}

// tokenize splits text into lowercase tokens, removing stop words and short words.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	result := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) < 2 || isStopWord(word) {
			continue
		}
		result = append(result, word)
	}
	return result
}

func isStopWord(word string) bool {
	_, ok := stopWords[word]
	return ok
}

//nolint:gochecknoglobals
var stopWords = map[string]struct{}{
	// English
	"the": {}, "a": {}, "an": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"be": {}, "been": {}, "being": {}, "have": {}, "has": {}, "had": {},
	"do": {}, "does": {}, "did": {}, "will": {}, "would": {}, "could": {},
	"should": {}, "may": {}, "might": {}, "must": {}, "shall": {},
	"to": {}, "of": {}, "in": {}, "for": {}, "on": {}, "with": {},
	"at": {}, "by": {}, "from": {}, "as": {}, "into": {}, "through": {},
	"about": {}, "up": {}, "out": {}, "than": {}, "and": {}, "but": {},
	"or": {}, "if": {}, "then": {}, "so": {}, "not": {}, "no": {},
	"it": {}, "its": {}, "this": {}, "that": {}, "these": {}, "those": {},
	"he": {}, "she": {}, "they": {}, "we": {}, "you": {}, "me": {},
	"him": {}, "her": {}, "us": {}, "them": {}, "my": {}, "his": {},
	"their": {}, "our": {}, "your": {}, "who": {}, "which": {}, "when": {},
	"where": {}, "how": {}, "what": {}, "all": {}, "each": {}, "every": {},
	"both": {}, "few": {}, "more": {}, "most": {}, "other": {}, "some": {},
	"such": {}, "only": {}, "own": {}, "same": {}, "also": {}, "just": {},
	"after": {}, "before": {}, "between": {}, "during": {}, "while": {},
	// Ukrainian
	"і": {}, "та": {}, "що": {}, "як": {}, "але": {}, "це": {},
	"він": {}, "вона": {}, "вони": {}, "ми": {}, "ви": {},
	"на": {}, "за": {}, "до": {}, "від": {}, "про": {}, "під": {},
	"над": {}, "між": {}, "через": {}, "після": {}, "перед": {},
	"з": {}, "із": {}, "зі": {}, "у": {}, "в": {},
	"не": {}, "ні": {}, "так": {}, "ще": {}, "вже": {},
	"його": {}, "її": {}, "їх": {}, "свій": {}, "свого": {},
	"який": {}, "яка": {}, "яке": {}, "які": {},
	"коли": {}, "де": {}, "хто": {}, "чого": {},
	"бути": {}, "було": {}, "була": {}, "були": {},
	"може": {}, "треба": {}, "потрібно": {},
	"той": {}, "ця": {}, "те": {}, "ті": {},
}
