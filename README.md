fz
===

fz performs a fuzzy prefix search against a line-delimited list of strings read
from stdin.

I use this code as a way to experiment with approaches to fuzzy prefix
searching. Although it works, there are other more battle-tested programs out
there if you're looking for a real fuzzy search tool. The current search
algorithm has exponential runtime, and will choke on really large inputs.

Examples
--------

fz searches a list of strings, allowing arbitrary wildcard gaps between each
character in the search term. Input strings that contain a portion of the
search term are ranked by: 1) how many characters match the term, 2) the number
of gaps in between matching characters, and 3) the length of the input string.
It works similarly to the command palette in Sublime Text or VSCode.

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
