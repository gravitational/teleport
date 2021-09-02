package diceware

// WordList is an interface that must be implemented to be considered a word
// list for use in the diceware algorithm. This interface can be implemented by
// other libraries.
type WordList interface {
	// Digits is the number of digits for indexes in the word list. This
	// determines the number of dice rolls.
	Digits() int

	// WordAt returns the word at the given integer in the word list.
	WordAt(int) string
}

var _ WordList = (*wordListInternal)(nil)

type wordListInternal struct {
	digits int
	words  map[int]string
}

func (w *wordListInternal) Digits() int {
	return w.digits
}

func (w *wordListInternal) WordAt(i int) string {
	return w.words[i]
}
