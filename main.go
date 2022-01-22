package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
)

// maxResults limits the results to the top N matches.
const maxResults = 25

func printUsage(w io.StringWriter) {
	w.WriteString(`usage: fz <search>

fz performs a fuzzy prefix search against a line-delimited list of strings read
from stdin.

Examples:

	# recursively search for file paths containing ".go"
	$ find . | fz .go
	./main.go
	./main_test.go
	./templates/index.gohtml
	./go.mod

	# search a list for the characters "p" and "l" anywhere in each string
	$ echo 'people
		person
		place
		ply
		dog' | fz 'pl'
	ply
	place
	people
	person
`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage(os.Stderr)
		os.Exit(1)
	}
	search := os.Args[1]
	switch search {
	case "-h", "-help", "--help":
		printUsage(os.Stdout)
		os.Exit(0)
	}

	s := newSearcher(search)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		s.append(scanner.Text())
	}
	for _, r := range s.rankedResults(maxResults) {
		r.printHighlight(os.Stdout)
	}
}

type searcher struct {
	term string

	batch        []string
	batchBytes   int
	batchByteMin int
	batchCount   int
	batchSem     chan struct{}
	batchResults chan []result
}

func newSearcher(term string) searcher {
	return searcher{
		term:         term,
		batchByteMin: 256000,
		batchSem:     make(chan struct{}, runtime.NumCPU()),
		batchResults: make(chan []result),
	}
}

func (s *searcher) append(input ...string) {
	for _, elem := range input {
		elem = strings.TrimSpace(elem)
		if elem == "" {
			return
		}

		s.batch = append(s.batch, elem)
		s.batchBytes += len(elem)
		if s.batchBytes >= s.batchByteMin {
			s.batchSem <- struct{}{}
			s.batchCount++
			go func(batch []string) {
				results := make([]result, len(batch))
				for i, b := range batch {
					elemResults := search(b, s.term, 0, nil)
					if len(elemResults) == 0 {
						continue
					}
					sort.Sort(byRank(elemResults))
					results[i] = elemResults[0]
				}
				<-s.batchSem
				s.batchResults <- results
			}(s.batch)
			s.batch = make([]string, 0, cap(s.batch))
			s.batchBytes = 0
		}
	}
}

func (s *searcher) rankedResults(max int) []result {
	close(s.batchSem)

	all := byRank([]result{})
	if len(s.batch) > 0 {
		for _, b := range s.batch {
			elemResults := search(b, s.term, 0, nil)
			if len(elemResults) == 0 {
				continue
			}
			sort.Sort(byRank(elemResults))
			all = append(all, elemResults[0])
		}
	}

	for i := 0; i < s.batchCount; i++ {
		all = append(all, <-s.batchResults...)
	}
	sort.Sort(all)
	if len(all) > max {
		return all[:max]
	}
	return all
}

// search performs a recursive fuzzy search for a term in s.
func search(s, term string, offset int, all []result) []result {
	// We're at the end of the input; nothing more to search.
	if offset == len(s) {
		return all
	}

	// Only search the part of the input after the offset.
	tail := s[offset:]
	res := result{input: s}
	for _, r := range term {
		i := strings.IndexRune(tail, r)
		if i == -1 {
			break
		}

		// Check if there was a gap between the previous rune match and
		// this rune match. If we didn't advance, then there's no gap
		// and we increment the last span. Otherwise, start a new span
		// at the current position.
		if i == 0 {
			if len(res.matches) == 0 {
				res.matches = append(res.matches, span{
					start: offset,
					end:   offset + 1,
				})
			} else {
				res.matches[len(res.matches)-1].end++
			}
		} else {
			res.matches = append(res.matches, span{
				start: offset + i,
				end:   offset + i + 1},
			)
		}

		i++
		tail = tail[i:]
		offset += i
	}

	// If the score is 0 then we didn't find anything, so don't bother
	// returning a match.
	if res.matchScore() == 0 {
		return all
	}

	// Search the input again starting after the first matched rune. This
	// lets us find any better matches that start later in the input. For
	// example, in:
	//
	// s = CxxxAxxxTCAT
	// term = CAT
	//
	// the last 3 characters are the best match (it matches the term
	// perfectly without gaps). If we didn't recursively search the input,
	// then we would only match on the first 'C', 'A', and 'T', returning a
	// suboptimal match of "CxxxAxxxT".
	//
	// This yields an exponential runtime, but whatever let's see how it
	// goes.
	return search(s, term, res.matches[0].start+1, append(all, res))
}

// byRank sorts results by their match score, then gap score, then shortest
// length.
type byRank []result

func (r byRank) Len() int {
	return len(r)
}

func (r byRank) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r byRank) Less(i, j int) bool {
	if r[i].matchScore() == r[j].matchScore() {
		if r[i].gapScore() == r[j].gapScore() {
			return len(r[i].input) < len(r[j].input)
		}
		return r[i].gapScore() > r[j].gapScore()
	}
	return r[i].matchScore() > r[j].matchScore()
}

// span is a range of runes in a string.
type span struct{ start, end int }

// result contains the matches from a search.
type result struct {
	// input is the string that was searched.
	input string

	// matches contains the spans within the input string where matching
	// runes were found.
	matches []span
}

// matchScore is how well the result matches the search term. The score
// increases for each search term rune that was found in the input.
func (r result) matchScore() int {
	score := 0
	for _, s := range r.matches {
		score += s.end - s.start
	}
	return score
}

// gapScore is a negative value that corresponds to how many gaps must be
// inserted into the search term to find a match.
func (r result) gapScore() int {
	return -len(r.matches) + 1
}

// printHighlight writes the result's input with matching runes bolded and
// colored.
func (r result) printHighlight(w io.Writer) {
	escLen := 4
	buf := bytes.Buffer{}
	buf.Grow(len(r.input) + len(r.matches)*2*escLen)
	inputPos := 0
	for _, m := range r.matches {
		n, _ := buf.WriteString(r.input[:m.start])
		inputPos += n

		buf.WriteString("\033[1m")

		n, _ = buf.WriteString(r.input[m.start:m.end])
		inputPos += n

		buf.WriteString("\033[0m")
	}
	if inputPos < len(r.input) {
		buf.WriteString(r.input[inputPos:])
	}
	buf.WriteByte('\n')
	buf.WriteTo(w)
}
