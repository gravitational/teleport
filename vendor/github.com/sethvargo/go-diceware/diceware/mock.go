package diceware

var _ DicewareGenerator = (*mockGenerator)(nil)

type mockGenerator struct {
	result []string
	err    error
}

// NewMockGenerator creates a new generator that satisfies the DicewareGenerator
// interface. If an error is provided, the error is returned. If a result if
// provided, the result is always returned, regardless of what parameters are
// passed into the Generate or MustGenerate methods.
//
// This function is most useful for tests where you want to have predicable
// results for a transitive resource that depends on go-diceware.
func NewMockGenerator(result []string, err error) *mockGenerator {
	return &mockGenerator{
		result: result,
		err:    err,
	}
}

// Generate returns the mocked result or error.
func (g *mockGenerator) Generate(int) ([]string, error) {
	if g.err != nil {
		return nil, g.err
	}
	return g.result, nil
}

// MustGenerate returns the mocked result or panics if an error was given.
func (g *mockGenerator) MustGenerate(int) []string {
	if g.err != nil {
		panic(g.err)
	}
	return g.result
}
