package regular

func Fuzz(data []byte) int {
	_, err := parseProxySubsysRequest(string(data))
	if err != nil {
		return 0
	}
	return 1
}
