package monger

// --- FILTER BUILDER ---
// Permite criar queries complexas sem usar a sintaxe verbosa do BSON
type FilterBuilder struct {
	f M
}

func Filter() *FilterBuilder {
	return &FilterBuilder{f: M{}}
}

// Comparadores
// Eq é um alias curto para Equal.
func (b *FilterBuilder) Eq(field string, val any) *FilterBuilder { return b.Equal(field, val) }

// Equal adiciona um comparador de igualdade: {field: val}
func (b *FilterBuilder) Equal(field string, val any) *FilterBuilder { b.f[field] = val; return b }

// Ne é um alias curto para NotEqual.
func (b *FilterBuilder) Ne(field string, val any) *FilterBuilder { return b.NotEqual(field, val) }

// NotEqual adiciona um comparador de diferença: {field: {$ne: val}}
func (b *FilterBuilder) NotEqual(field string, val any) *FilterBuilder {
	b.f[field] = M{"$ne": val}
	return b
}

// Gt é um alias curto para GreaterThan.
func (b *FilterBuilder) Gt(field string, val any) *FilterBuilder { return b.GreaterThan(field, val) }

// GreaterThan adiciona um comparador maior que: {field: {$gt: val}}
func (b *FilterBuilder) GreaterThan(field string, val any) *FilterBuilder {
	b.f[field] = M{"$gt": val}
	return b
}

// Gte é um alias curto para GreaterThanOrEqual.
func (b *FilterBuilder) Gte(field string, val any) *FilterBuilder {
	return b.GreaterThanOrEqual(field, val)
}

// GreaterThanOrEqual adiciona um comparador maior ou igual: {field: {$gte: val}}
func (b *FilterBuilder) GreaterThanOrEqual(field string, val any) *FilterBuilder {
	b.f[field] = M{"$gte": val}
	return b
}

// Lt é um alias curto para LessThan.
func (b *FilterBuilder) Lt(field string, val any) *FilterBuilder { return b.LessThan(field, val) }

// LessThan adiciona um comparador menor que: {field: {$lt: val}}
func (b *FilterBuilder) LessThan(field string, val any) *FilterBuilder {
	b.f[field] = M{"$lt": val}
	return b
}

// Lte é um alias curto para LessThanOrEqual.
func (b *FilterBuilder) Lte(field string, val any) *FilterBuilder {
	return b.LessThanOrEqual(field, val)
}

// LessThanOrEqual adiciona um comparador menor ou igual: {field: {$lte: val}}
func (b *FilterBuilder) LessThanOrEqual(field string, val any) *FilterBuilder {
	b.f[field] = M{"$lte": val}
	return b
}

// In é um alias curto para InValues.
func (b *FilterBuilder) In(field string, vals any) *FilterBuilder { return b.InValues(field, vals) }

// InValues adiciona um comparador "IN": {field: {$in: vals}}
func (b *FilterBuilder) InValues(field string, vals any) *FilterBuilder {
	b.f[field] = M{"$in": vals}
	return b
}

// Operadores Lógicos (And / Or)
func (b *FilterBuilder) And(builders ...*FilterBuilder) *FilterBuilder {
	filters := []M{}
	for _, sub := range builders {
		filters = append(filters, sub.Build())
	}
	b.f["$and"] = filters
	return b
}

func (b *FilterBuilder) Or(builders ...*FilterBuilder) *FilterBuilder {
	filters := []M{}
	for _, sub := range builders {
		filters = append(filters, sub.Build())
	}
	b.f["$or"] = filters
	return b
}

func (b *FilterBuilder) Build() M {
	return b.f
}
