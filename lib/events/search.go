package events

func Index(s string, query string) int {
	d := calculateSlideTable(query)
	return IndexWithTable(d, s, query)
}

// IndexWithTable returns the first index query found in the s.
// It needs the slide information of query
func IndexWithTable(table [256]int, contents string, query string) int {
	queryLen := len(query)
	contentsLen := len(contents)
	switch {
	case queryLen == 0:
		return 0
	case queryLen > contentsLen:
		return -1
	case queryLen == contentsLen:
		if contents == query {
			return 0
		}
		return -1
	}

	i := 0
	for i+queryLen-1 < contentsLen {
		j := queryLen - 1
		for ; j >= 0 && contents[i+j] == query[j]; j-- {
		}
		if j < 0 {
			return i
		}

		slid := max(1, j-table[contents[i+j]])
		i += slid
	}
	return -1
}

// calculateSlideTable builds sliding amount per each unique byte in the query
func calculateSlideTable(query string) [256]int {
	var d [256]int
	for i := 0; i < 256; i++ {
		d[i] = -1
	}
	for i := 0; i < len(query); i++ {
		d[query[i]] = i
	}
	return d
}
