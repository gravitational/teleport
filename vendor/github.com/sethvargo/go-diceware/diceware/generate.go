// Package diceware provides a library for generating random words via the
// diceware algorithm by rolling five six-sided dice to randomly select a word
// from a list of english words.
//
// Read more about the diceware algorithm here: https://en.wikipedia.org/wiki/Diceware.
//
//    list, err := diceware.Generate(6)
//    if err != nil  {
//      log.Fatal(err)
//    }
//    log.Printf(strings.Join(list, "-"))
//
// Most functions are safe for concurrent use.
package diceware

import (
	"crypto/rand"
	"math"
	"math/big"
)

// sides is the number of sides on a die
var sides = big.NewInt(6)

var _ DicewareGenerator = (*Generator)(nil)

// Generator is the stateful generator which can be used to customize the word
// list and other generation options.
type Generator struct {
	wordList WordList
}

// GeneratorInput is used as input to the NewGenerator function.
type GeneratorInput struct {
	// WordList is the word list to use. There are built-in word lists like
	// WordListEffBig (default), WordListEffSmall, and WordListOriginal. You can
	// also bring your own word list by implementing the WordList interface.
	WordList WordList
}

// NewGenerator creates a new Generator from the specified configuration. If no
// input is given, all the default values are used. This function is safe for
// concurrent use.
func NewGenerator(i *GeneratorInput) (*Generator, error) {
	if i == nil {
		i = new(GeneratorInput)
	}

	if i.WordList == nil {
		i.WordList = WordListEffLarge()
	}

	gen := &Generator{
		wordList: i.WordList,
	}

	return gen, nil
}

// Generate generates a collection of diceware words, specified by the numWords
// parameter.
//
// The algorithm is fast, but it's not designed to be performant, favoring
// entropy over speed.
//
// This function is safe for concurrent use, but there is a possibility of
// concurrent invocations generating overlapping words. To generate multiple
// non-overlapping words, use a single invocation of the function and split the
// resulting string list.
func (g *Generator) Generate(numWords int) ([]string, error) {
	list := make([]string, 0, numWords)
	seen := make(map[string]struct{}, numWords)

	for i := 0; i < numWords; i++ {
		n, err := RollWord(g.wordList.Digits())
		if err != nil {
			return nil, err
		}

		word := g.wordList.WordAt(n)
		if _, ok := seen[word]; ok {
			i--
			continue
		}

		list = append(list, word)
		seen[word] = struct{}{}
	}

	return list, nil
}

// MustGenerate is the same as Generate, but panics on error.
func (g *Generator) MustGenerate(numWords int) []string {
	list, err := g.Generate(numWords)
	if err != nil {
		panic(err)
	}
	return list
}

// Generate - see Generator.Generate for usage.
func Generate(numWords int) ([]string, error) {
	gen, err := NewGenerator(nil)
	if err != nil {
		return nil, err
	}
	return gen.Generate(numWords)
}

// MustGenerate - see Generator.MustGenerate for usage.
func MustGenerate(numWords int) []string {
	gen, err := NewGenerator(nil)
	if err != nil {
		panic(err)
	}
	return gen.MustGenerate(numWords)
}

// GenerateWithWordList generates a list of the given number of words from the
// given word list.
func GenerateWithWordList(numWords int, wordList WordList) ([]string, error) {
	gen, err := NewGenerator(&GeneratorInput{
		WordList: wordList,
	})
	if err != nil {
		return nil, err
	}
	return gen.Generate(numWords)
}

// WordAt retrieves the word at the given index from EFF's large wordlist.
//
// This function is DEPRECATED. Please use WordList.WordAt instead.
func WordAt(i int) string {
	return WordListEffLarge().WordAt(i)
}

// RollDie rolls a single 6-sided die and returns a value between [1,6].
func RollDie() (int, error) {
	r, err := rand.Int(rand.Reader, sides)
	if err != nil {
		return 0, err
	}
	return int(r.Int64()) + 1, nil
}

// RollWord rolls and aggregates dice to represent one word in the list. The
// result is the index of the word in the list.
func RollWord(d int) (int, error) {
	var final int

	for i := d; i > 0; i-- {
		res, err := RollDie()
		if err != nil {
			return 0, err
		}

		final += res * int(math.Pow(10, float64(i-1)))
	}

	return final, nil
}
