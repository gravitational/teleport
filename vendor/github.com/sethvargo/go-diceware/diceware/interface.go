package diceware

type DicewareGenerator interface {
	Generate(int) ([]string, error)
	MustGenerate(int) []string
}
