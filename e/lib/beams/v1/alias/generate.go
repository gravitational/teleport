package alias

import "math/rand/v2"

// Generate a human friendly alias for a beam, in the form of an `adjective-noun`
// pair consisting of random sci-fi-related words, selected for ease-of-typing by
// GPT 5.3, and carefully reviewed to remove offensive or triggering combinations.
//
// With the current corpus of words, there are ~32.4K possible combinations, so
// the likelihood of a collision is much higher than a UUID or other more random
// identifier, and you may need to retry a few times to find a unique alias.
func Generate() (string, error) {
	return randomWord(adjectives) + "-" + randomWord(nouns), nil
}

func randomWord(words []string) string {
	return words[rand.IntN(len(words))]
}
