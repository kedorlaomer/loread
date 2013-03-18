package nntp

// implements the algorithm from http://www.jwz.org/doc/threading.html

// Tree-structured wrapper around FormattedArticle.
type Container struct {
	article             FormattedArticle // underlying article
	parent, child, next *Container       // link structure (threaded tree)
}

var id_table = make(map[MessageId]Container)

// Threads articles according to the algorithm from
// http://www.jwz.org/doc/threading.html; see also
// https://raw.github.com/kedorlaomer/loread/master/threading.txt
func Thread(articles []FormattedArticle) []Container {
	// 1. for each message…
	for _, message := range articles {
		// A. insert to id_table
        container := Container{
			article: message,
		}

		id_table[message.Id] = container
	}

    // The original algorithm says here that steps 1A and 1B
    // should be performed in the same loop (for each message),
    // but using the same loop seems to be easier.
    for _, message := range articles {
        // B. follow references
        for  _, ref := range message.References {
            // TODO: finish
        }
    }

	return []Container{}
}
