package argus

// WordLev computes the Levenshtein distance between two slices of string (words)
// Used to compute the distance between two ASN sequences or link-level sequences
// We use word based edit distances since ASN123 and ASN1234 and ASN2832 are different ASNs who's change should be weighted the same
func WordLev(a, b []string) int {
	rows := len(a)
	cols := len(b)

	// table[i][j] = min edits to turn first i words of a into first j words of b
	table := make([][]int, rows+1)
	for i := range table {
		table[i] = make([]int, cols+1)
	}

	for i := 0; i <= rows; i++ {
		table[i][0] = i
	}
	for j := 0; j <= cols; j++ {
		table[0][j] = j
	}

	for i := 1; i <= rows; i++ {
		for j := 1; j <= cols; j++ {
			subCost := 1
			if a[i-1] == b[j-1] { // fixed: b[j-1], not b[i-1]
				subCost = 0
			}

			deleteWord := table[i-1][j] + 1
			insertWord := table[i][j-1] + 1
			replaceWord := table[i-1][j-1] + subCost

			table[i][j] = min(deleteWord, insertWord, replaceWord)
		}
	}

	return table[rows][cols]
}
