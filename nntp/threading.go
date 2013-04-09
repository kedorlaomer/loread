package nntp

import (
	"fmt"
	"sort"
	"strings"
)

// implements the algorithm from http://www.jwz.org/doc/threading.html

// Tree-structured wrapper around FormattedArticle.
type Container struct {
	Article             *ParsedArticle // underlying Article
	Parent, Child, Next *Container     // link structure (threaded tree)
}

type idTable map[MessageId]*Container
type subjectTable map[string]*Container

var id_table idTable

// Prefixes that typically mean a follow up. For technical
// reasons, this can't be a constant.
var _BAD_PREFIXES = []string{"re: ", "aw: "}

// Threads articles according to the algorithm from
// http://www.jwz.org/doc/threading.html; see also
// https://raw.github.com/kedorlaomer/loread/master/threading.txt
func Thread(articles []ParsedArticle) []*Container {
	id_table = make(idTable)

	// 1. Rough threading
	for i, message := range articles {
		// A. Insert to id_table
		container := &Container{
			Article: &articles[i],
		}

		id_table[message.Id] = container
	}

	// The original algorithm says here that steps 1A, 1B and 1C
	// should be performed in the same loop (for each message),
	// but using different loops seems to be easier.
	for _, message := range articles {
		// B. Link together what belongs together according to
		// message.References
		if len(message.References) > 1 {
			for i := len(message.References) - 2; i >= 0; i-- {
				id1 := message.References[i+1]
				id2 := message.References[i]
				c1 := containerById(id1)
				c2 := containerById(id2)

				// insert a link c1 → c2
				if c1.Parent == nil && mayLink(c1, c2) {
					c1.Parent = c2
				}
			}
		}
	}

	for _, container := range id_table {
		// C. Set message's parent.
		refs := container.Article.References
		if len(refs) > 0 {
			// parent, according to References
			realParent := containerById(refs[0])
			if container.Parent == nil {
				container.Parent = realParent
			}
		}
	}

	// Now, messages are linked according to parents. Insert
	// child links.
	for _, container := range id_table {
		if Parent := container.Parent; Parent != nil && Parent.Child == nil {
			Parent.Child = container
		}
	}

	// Children of the same parent should get linked via the
	// next pointer.
	for _, container := range id_table {
		if Parent := container.Parent; Parent != nil {
			Child := Parent.Child
			// If container is in the list of child→next→next→…,
			// we mustn't insert it again.
			mayInsert := Child != container
			for Child.Next != nil && mayInsert {
				mayInsert = mayInsert && Child != container
				Child = Child.Next
			}

			if mayInsert {
				Child.Next = container
			}
		}
	}

	// 2.
	rootSet := []*Container{}

	// containers without parents are the root set
	for _, container := range id_table {
		if container.Parent == nil {
			rootSet = append(rootSet, container)
		}
	}

	// 3.
	id_table = nil

	// 4. This has to use recursion.
	for _, container := range rootSet {
		pruneEmptyContainer(container, true)
	}

	// 5. Group root set by subject

	// 5A.
	subject_table := make(subjectTable)

	// 5B. Find root set's subjects
	for _, c := range rootSet {
		subject := stripPrefixes(findSubject(c))

		if subject == "" {
			continue
		}

		old, ok := subject_table[subject]
		// no old container → insert
		if !ok {
			subject_table[subject] = c
		} else {
			// new container is empty → more interesting
			if c.Article == nil {
				subject_table[subject] = c
			} else {
				if isFollowup(old.Article.Subject) && !isFollowup(c.Article.Subject) {
					subject_table[subject] = c
				}
			}
		}
	}

	// 5C. Merge containers with (almost) equal Subject.
	for i, this := range rootSet {
		subject := stripPrefixes(findSubject(this))

		if subject == "" {
			continue
		}

		that := subject_table[subject]

		if that == nil || that == this {
			continue
		}

		// We want to merge the separate threads this and that.

		// (i) both dummies: append that's children to this'
		// children
		if this.Article == nil && that.Article == nil {
			last := this
			for last.Next != nil {
				last = last.Next
			}

			last.Next = that.Child
			that.Child = nil
			that.Next = nil
			delete(subject_table, subject)
			subject_table[subject] = this

			continue
		}

		// (ii) one is empty, the that isn't → append non-empty
		if (this.Article == nil) != (that.Article == nil) {
			var empty, nonEmpty *Container
			if this.Article == nil {
				empty = this
				nonEmpty = that
			} else {
				empty = that
				nonEmpty = this
			}

			last := empty
			for last.Next != nil {
				last = last.Next
			}

			last.Next = nonEmpty

			continue
		}

		// (iii) that is non-empty, that is not a follow up, but
		// this is → make this child of that
		if that.Article != nil && !isFollowup(that.Article.Subject) &&
			isFollowup(this.Article.Subject) {
			last := that
			for last.Next != nil {
				last = last.Next
			}

			last.Next = this

			continue
		}

		// (iv) that is non-empty, that is follow up, but this
		// is isn't → make that child of this
		if that.Article != nil && isFollowup(that.Article.Subject) &&
			!isFollowup(this.Article.Subject) {
			last := this
			for last.Next != nil {
				last = last.Next
			}

			last.Next = that

			continue
		}

		// (v) make this and that siblings of a new container
		last := this
		for last.Next != nil {
			last = last.Next
		}

		last.Next = that

		rootSet[i] = &Container{
			Child: this,
		}
	}

	// 6. Done.

	// 7. Not really: Need to sort siblings. This is is easier
	// to do recursively.
	for _, c := range rootSet {
		sortSiblings(c)
	}

	for _, c := range rootSet {
		printContainers(c)
	}

	return rootSet
}

// id_table[id] may be nil, but we want an empty container
// instead.
func containerById(id MessageId) *Container {
	rv := id_table[id]
	if rv == nil {
		return &Container{}
	}

	return rv
}

// Is c2 reachable from c1? FIXME: Do we need a graph traversal?
func (c1 *Container) reachable(c2 *Container) bool {
	// Breadth-first traversal of graph structure generated by
	// edges parent, child, next.
	visited := make(map[*Container]bool)
	q := NewQueue()
	q.Enqueue(c1)

	for !q.Empty() {
		if c := q.Dequeue().(*Container); c != nil && !visited[c] {
			visited[c] = true
			q.Enqueue(c.Parent)
			q.Enqueue(c.Child)
			q.Enqueue(c.Next)
			if c == c2 { // c2 is reachable
				return true
			}
		}
	}

	return false
}

// Is adding a link between c1 and c2 allowed?
func mayLink(c1, c2 *Container) bool {
	return !c1.reachable(c2) && !c2.reachable(c1)
}

// for debugging: displays the link structure of c
func printContainers(c *Container) {
	printContainersRek(c, 0)
}

func printContainersRek(c *Container, depth int) {
	if depth > 0 {
		for i := 0; i < depth; i++ {
			fmt.Print("•")
		}
	}

	if c.Article != nil {
		fmt.Println(c.Article.Subject)
	} else {
		fmt.Println("<<empty container>>")
	}

	for c2 := c.Child; c2 != nil; c2 = c2.Next {
		printContainersRek(c2, depth+1)
	}
}

// Recursively perform step 4, following „Next“ and „Child“
// links. The flag isRoot is true, if c is in the rootset (in
// this case, only single children should be promoted to the
// root).
func pruneEmptyContainer(c *Container, isRoot bool) {
	if c == nil {
		return
	}

	// 4A. empty article, no children
	if shouldNuke(c.Child) {
		c.Child = nil
	}

	if shouldNuke(c.Next) {
		c.Next = nil
	}

	// last sibling of c
	last := c
	for last.Next != nil {
		last = last.Next
	}

	// 4B. empty article, has children → promote
	if c.Child != nil && c.Child.Article == nil {
		if !isRoot || c.Child.Next == nil {
			last.Next = c.Child
		}
	}

	// recurse
	pruneEmptyContainer(c.Child, false)
	pruneEmptyContainer(c.Next, false)
}

func shouldNuke(c *Container) bool {
	return c != nil && c.Child == nil && c.Next == nil && c.Article == nil
}

// removes leading _BAD_PREFIXES from subj
func stripPrefixes(subj string) string {
	redo := true
	for redo {
		redo = false
		for _, prefix := range _BAD_PREFIXES {
			if strings.HasPrefix(strings.ToLower(subj), prefix) {
				subj = subj[len(prefix):]
				redo = true
			}
		}
	}

	return subj
}

// does subj start with one of the typical follow-up prefixes?
func isFollowup(subj string) bool {
	for _, prefix := range _BAD_PREFIXES {
		if strings.HasPrefix(strings.ToLower(subj), prefix) {
			return true
		}
	}

	return false
}

// uses the procedure described in 5B for finding the subject of
// c's article
func findSubject(c *Container) string {
	if article1 := c.Article; article1 == nil {
		if c.Child == nil || c.Child.Article == nil || c.Child.Article.Subject == "" {
			return ""
		}
		return c.Child.Article.Subject
	} else {
		return article1.Subject
	}

	return ""
}

// infrastructure for sorting []*Container by date
type containers []*Container

func (c containers) Less(i, j int) bool {
	return c[i].Article.Date.Before(c[j].Article.Date)
}

func (c containers) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c containers) Len() int {
	return len(c)
}

// sorts c.Child, c.Child.Next, … by their date header
func sortSiblings(c *Container) {
	siblings := []*Container{}

	if c.Child != nil {
		for s := c.Child; s != nil; s = s.Next {
			siblings = append(siblings, s)
		}

		if len(siblings) > 1 {
			sort.Sort(containers(siblings))

			// insert to linked list
			c.Child = siblings[0]
			iter := c.Child

			for _, s := range siblings[1:] {
				iter.Next = s
				iter = s
			}

			// terminate list
			iter.Next = nil
		}
	}
}
