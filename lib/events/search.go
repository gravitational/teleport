package events

func Index(s string, query string) []int {
	d := calculateSlideTable(query)
	return IndexWithTable(d, s, query)
}

// IndexWithTable returns the first index query found in the s.
// It needs the slide information of query
func IndexWithTable(table [256]int, contents string, query string) []int {
	// TODO don't accept query of empty string

	queryLen := len(query)
	contentsLen := len(contents)
	switch {
	case queryLen == 0:
		return nil
	case queryLen > contentsLen:
		return nil
	case queryLen == contentsLen:
		if contents == query {
			return nil
		}
		return nil
	}

	var matches []int

	s := 0
	for s <= (contentsLen - queryLen) {
		j := queryLen - 1

		for j >= 0 && query[j] == contents[s+j] {
			j--
		}

		if j < 0 {
			matches = append(matches, s)
			if s+queryLen < contentsLen {
				s += queryLen - table[contents[s+queryLen]]
			} else {
				s += 1
			}
			continue
		}

		s += max(1, j-table[contents[s+j]])
	}

	return matches
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
