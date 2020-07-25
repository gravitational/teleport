package utils

func Fuzz(data []byte) int {
	_, err := ParseProxyJump(string(data))
	if err != nil {
		return 0
	}
	return 1
}
