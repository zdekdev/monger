package monger

// --- PROJECT BUILDER ---
// Controla quais campos serão retornados (SELECT no SQL)
type ProjectBuilder struct {
	p M
}

func Select(fields ...string) *ProjectBuilder {
	m := M{}
	for _, f := range fields {
		m[f] = 1
	}
	return &ProjectBuilder{p: m}
}

func Exclude(fields ...string) *ProjectBuilder {
	m := M{}
	for _, f := range fields {
		m[f] = 0
	}
	return &ProjectBuilder{p: m}
}

func (b *ProjectBuilder) Build() M {
	return b.p
}
