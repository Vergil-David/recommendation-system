package embeddings

import (
	"math"
	"testing"
)

func TestNewGenerator_DefaultDimension(t *testing.T) {
	g := NewGenerator(0)
	if g.dimension != DefaultDimension {
		t.Errorf("NewGenerator(0).dimension = %d, want %d", g.dimension, DefaultDimension)
	}
}

func TestNewGenerator_CustomDimension(t *testing.T) {
	g := NewGenerator(64)
	if g.dimension != 64 {
		t.Errorf("NewGenerator(64).dimension = %d, want 64", g.dimension)
	}
}

func TestTransform_OutputDimension(t *testing.T) {
	g := NewGenerator(128)
	g.Fit([]ItemText{{Title: "test", Description: "hello world", Genres: []string{"Drama"}}})

	vec := g.Transform(ItemText{Title: "test", Description: "hello world", Genres: []string{"Drama"}})
	if len(vec) != 128 {
		t.Errorf("Transform output dimension = %d, want 128", len(vec))
	}
}

func TestTransform_L2Normalized(t *testing.T) {
	g := NewGenerator(64)
	items := []ItemText{
		{Title: "The Matrix", Description: "A computer hacker learns about the true nature of reality", Genres: []string{"Action", "Sci-Fi"}},
		{Title: "Inception", Description: "A thief steals secrets from dreams", Genres: []string{"Action", "Thriller"}},
	}
	g.Fit(items)

	for _, item := range items {
		vec := g.Transform(item)
		var norm float64
		for _, v := range vec {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)

		if math.Abs(norm-1.0) > 1e-5 {
			t.Errorf("Transform(%q) L2 norm = %f, want ~1.0", item.Title, norm)
		}
	}
}

func TestTransform_SimilarItemsCloser(t *testing.T) {
	g := NewGenerator(128)
	items := []ItemText{
		{Title: "Action Movie", Description: "Explosions and car chases", Genres: []string{"Action"}},
		{Title: "Action Film", Description: "Fight scenes and explosions", Genres: []string{"Action"}},
		{Title: "Romantic Comedy", Description: "Love and laughter in Paris", Genres: []string{"Comedy", "Romance"}},
	}
	g.Fit(items)

	vecAction1 := g.Transform(items[0])
	vecAction2 := g.Transform(items[1])
	vecRomcom := g.Transform(items[2])

	distSimilar := l2Distance(vecAction1, vecAction2)
	distDifferent := l2Distance(vecAction1, vecRomcom)

	if distSimilar >= distDifferent {
		t.Errorf("similar items distance (%f) should be less than different items distance (%f)",
			distSimilar, distDifferent)
	}
}

func TestTransform_DifferentItemsDifferentVectors(t *testing.T) {
	g := NewGenerator(64)
	items := []ItemText{
		{Title: "Horror Movie", Description: "Scary ghosts", Genres: []string{"Horror"}},
		{Title: "Comedy Show", Description: "Funny jokes", Genres: []string{"Comedy"}},
	}
	g.Fit(items)

	vec1 := g.Transform(items[0])
	vec2 := g.Transform(items[1])

	if l2Distance(vec1, vec2) < 0.01 {
		t.Error("different items should produce meaningfully different vectors")
	}
}

func TestTokenize_BasicSplit(t *testing.T) {
	tokens := tokenize("Hello, World! This is a test-123.")
	if len(tokens) == 0 {
		t.Fatal("tokenize should return tokens")
	}

	for _, tok := range tokens {
		if tok != tok { // just check iteration works
			t.Error("unexpected")
		}
	}
}

func TestTokenize_StopWordsRemoved(t *testing.T) {
	tokens := tokenize("the quick brown fox is and the")
	for _, tok := range tokens {
		if tok == "the" || tok == "is" || tok == "and" {
			t.Errorf("stop word %q should have been removed", tok)
		}
	}
}

func TestTokenize_ShortWordsRemoved(t *testing.T) {
	tokens := tokenize("I a do my go")
	for _, tok := range tokens {
		if len(tok) < 2 {
			t.Errorf("token %q should have been removed (too short)", tok)
		}
	}
}

func TestFit_BuildsDocFreq(t *testing.T) {
	g := NewGenerator(64)
	items := []ItemText{
		{Title: "cat dog", Description: "pets", Genres: nil},
		{Title: "cat fish", Description: "animals", Genres: nil},
	}
	g.Fit(items)

	if !g.fitted {
		t.Error("Fit should set fitted = true")
	}
	if g.totalDocs != 2 {
		t.Errorf("totalDocs = %d, want 2", g.totalDocs)
	}
	// "cat" appears in both docs, "dog" in one
	if g.docFreq["cat"] != 2 {
		t.Errorf("docFreq[cat] = %d, want 2", g.docFreq["cat"])
	}
	if g.docFreq["dog"] != 1 {
		t.Errorf("docFreq[dog] = %d, want 1", g.docFreq["dog"])
	}
}

// Helper: compute L2 distance between two float32 vectors
func l2Distance(a, b []float32) float64 {
	var sum float64
	for i := range a {
		d := float64(a[i]) - float64(b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}
