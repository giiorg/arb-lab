// Package domain stands in for another bounded context's domain. Rule 1 says
// nothing about one domain importing another — that is a context-mapping
// question, not a layering one — so an import of this must stay clean.
package domain

type Limit struct{}
