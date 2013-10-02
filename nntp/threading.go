package nntp

import (
	"fmt"
	//"math/rand"
	"sort"
	"strings"
)

// implements the algorithm from http://www.jwz.org/doc/threading.html

// Tree-structured wrapper around ParsedArticle.
type Container struct {
	Article             *ParsedArticle // underlying Article
	Parent, Child, Next *Container     // link structure (threaded tree)
	Id                  MessageId      // its Article's ID (makes sense if we don't have this article)
	Secondary           *Container     // next container in breadth-first traversal
}

// Result of depth-first walking a Container tree and noting the
// depths.

type DepthContainer struct {
	Cont *Container // visited Cont
	D    int        // at depth depth
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
func Thread(articles []ParsedArticle) map[*Container]bool {
	id_table = make(idTable)

	// 1
	for _, message := range articles {
		// 1A
		container := containerById(message.Id)

		if container.Article == nil {
			container.Article = new(ParsedArticle)
			*container.Article = message
		}

		// 1B
		for i := 0; i < len(message.References)-1; i++ {
			container1 := containerById(message.References[i])
			container2 := containerById(message.References[i+1])

			if container1.Parent == nil && container2.Parent == nil &&
				mayLink(container1, container2) {
				container2.Parent = container1
			}
		}

		// 1C
		if l := len(message.References); l > 0 {
			if last := containerById(message.References[l-1]); mayLink(container, last) {
				container.Parent = last
			}
		} else {
			container.Parent = nil
		}
	}

	// we „forgot“ to set Child and Next links

	// Child links
	for _, container := range id_table {
		if parent := container.Parent; parent != nil && parent.Child == nil {
			parent.Child = container
		}
	}

	// Next links
	for _, container := range id_table {
		if parent := container.Parent; parent != nil && parent.Child != container {
			otherChild := parent.Child
			for otherChild.Next != nil && otherChild != container {
				otherChild = otherChild.Next
			}

			if otherChild != container {
				otherChild.Next = container
			}
		}
	}

	// 2
	rootSet := make(map[*Container]bool)

	for _, message := range articles {
		container := containerById(message.Id)

		for container.Parent != nil {
			container = container.Parent
		}

		rootSet[container] = true
	}

	// 3
	id_table = nil

	// 4
	//
	// we use WalkContainers as replacement for recursion

	repeat := false

	// for whatever reason, doing this once isn't sufficient
	for repeat {
		repeat = false
		ch := make(chan *DepthContainer)
		go WalkContainers(rootSet, ch)

		for d := range ch {
			container := d.Cont
			// 4A
			if container.Article == nil {
				if container.Child == nil && container.Next == nil {
					delete(rootSet, container)
					repeat = true
					// delete from parent's child list, if existing
					deleteFromParentsList(container)
				}
			}

			// 4B
			if container.Article == nil && container.Child != nil {
				// remove this container
				repeat = true
				delete(rootSet, container)

				// promote single child to root set
				if container.Child.Next == nil {
					rootSet[container.Child] = true
				} else if container.Parent != nil {
					// promote non-single child to non-root
					parent := container.Parent
					last := parent.Child
					for last.Next != nil {
						last = last.Next
					}

					last.Next = container.Child
				}
			}
		}
	}

	// 5

	// A
	subject_table := make(subjectTable)

	// B
	for this := range rootSet {
		subject := findSubject(this)
		if subject != "" {
			old, ok := subject_table[subject]
			if !ok ||
				(this.Article == nil || old.Article != nil) ||
				isFollowup(old.Article.Subject) && !isFollowup(this.Article.Subject) {
				subject_table[subject] = this
			}
		}
	}

	// C
	////for this := range rootSet {
	////	subject := findSubject(this)
	////	that, ok := subject_table[subject]
	////	if !ok || this == that {
	////		continue
	////	}

	////	// (a)
	////	// both are dummies
	////	if this.Article == nil && that.Article == nil {
	////		// append this' children to that's children
	////		last := that.Child
	////		for last.Next != nil {
	////			last = last.Next
	////		}

	////		last.Next = this.Child

	////		// and delete this
	////		delete(rootSet, this)
	////		subject_table[subject] = that
	////	} else if ((this.Article == nil) && (that.Article != nil)) ||
	////		((this.Article != nil) && (that.Article == nil)) {
	////		// (b)
	////		// one is empty, another one is not
	////		if this.Article == nil {
	////			this, that = that, this
	////		}

	////		// that is empty, this isn't

	////		subject_table[subject] = that
	////		makeToChildOf(this, that)

	////	} else if that.Article != nil && !isFollowup(that.Article.Subject) &&
	////		this.Article != nil && isFollowup(this.Article.Subject) {
	////		// (c)
	////		// that is a follow-up, this isn't
	////		makeToChildOf(this, that)
	////		subject_table[subject] = that
	////	} else if that.Article != nil && isFollowup(that.Article.Subject) &&
	////		this.Article != nil && !isFollowup(this.Article.Subject) {
	////		// (d)
	////		// misordered
	////		makeToChildOf(that, this)
	////	} else {
	////		// (e)
	////		// otherwise
	////		newId := fmt.Sprintf("id%s@random.id", rand.Int())

	////		container := &Container{
	////			Id: MessageId(newId),
	////		}

	////		// container
	////		//    ↓
	////		//  this→⋯→last→that

	////		this.Parent = container
	////		that.Parent = container

	////		container.Child = this
	////		last := this
	////		for last.Next != nil {
	////			last = last.Next
	////		}

	////		last.Next = that
	////	}
	////}

	// 6 (nothing)

	// 7

	ch := make(chan *DepthContainer)
	go WalkContainers(rootSet, ch)

	for container := range ch {
		sortSiblings(container.Cont)
	}

	// the algorithm ends here; we need additional work

	// add Secondary links according to depth-first traversal
	ch = make(chan *DepthContainer)
	go WalkContainers(rootSet, ch)

	var first *DepthContainer
	for first = range ch {
		if first != nil && first.Cont != nil {
			break
		}
	}

	for second := range ch {
		if second == nil || second.Cont == nil || second.Cont.Article != nil {
			first.Cont.Secondary = second.Cont
			first = second
		}
	}

	return rootSet
}

// id_table[id] may be nil, but we want an empty container
// instead.
func containerById(id MessageId) *Container {
	rv := id_table[id]
	if rv == nil {
		rv = &Container{Id: id}
		id_table[id] = rv
	}

	return rv
}

// Is c2 reachable from c1 by using Parent links?
func (c1 *Container) reachableUpwards(c2 *Container) bool {
	c := c1.Parent

	for c != nil {
		if c == c2 {
			return true
		}

		c = c.Parent
	}
	return false
}

// Is adding a link between c1 and c2 allowed?
func mayLink(c1, c2 *Container) bool {
	return c1 != c2 && !c1.reachableUpwards(c2) && !c2.reachableUpwards(c1)
}

// for debugging: displays the link structure of container
func printContainers(container *Container) {
	c := container
	for c != nil {
		printContainersRek(c, 0)
		c = c.Next
	}
}

func printContainersRek(c *Container, depth int) {
	if c == nil {
		return
	}

	if depth > 0 {
		for i := 0; i < depth; i++ {
			fmt.Print("•")
		}
	}

	if c.Article != nil {
		fmt.Printf("%s (%s)\n", c.Article.Subject, c.Id)
	} else {
		fmt.Printf("<<empty container>> (%s)\n", c.Id)
	}

	for c2 := c.Child; c2 != nil; c2 = c2.Next {
		printContainersRek(c2, depth+1)
	}
}

// writes containers in breadth-first order to ch; closes ch
func WalkContainers(containers map[*Container]bool, ch chan<- *DepthContainer) {
	for container := range containers {
		c := container
		for c != nil {
			walkContainersRek(c, ch, 0)
			c = c.Next
		}
	}

	close(ch)
}

// recursive kernel for walkContainers
func walkContainersRek(container *Container, ch chan<- *DepthContainer, depth int) {
	if container == nil {
		return
	}

	ch <- &DepthContainer{
		Cont: container,
		D:    depth,
	}

	for c := container.Child; c != nil; c = c.Next {
		walkContainersRek(c, ch, depth+1)
	}
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

// nil is smaller than anything
func (c containers) Less(i, j int) bool {
	if c[i] == nil || c[i].Article == nil {
		return true
	}

	if c[j] == nil || c[j].Article == nil {
		return false
	}

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

func deleteFromParentsList(container *Container) {
	if parent := container.Parent; parent != nil {
		if container == parent.Child {
			parent.Child = nil
		} else {
			find := parent.Child
			for find.Next != container {
				find = find.Next
			}

			find.Next = nil
		}
	}

}

// make this to a child of that
func makeToChildOf(this, that *Container) {
	// make this a child of that and a sibling of that's children

	// this  that           that
	//        ↓         ⇒    ↓
	//        c→⋯→last       c→⋯→last→this

	this.Parent = that
	last := that.Child
	for last.Next != nil {
		last = last.Next
	}

	// relink this' siblings' Parent links to that
	last.Next = this
	for this != nil {
		this.Parent = that
		this = this.Next

	}
}
