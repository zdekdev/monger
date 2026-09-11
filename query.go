package monger

import "context"

// --- QUERY BUILDER ---

// Query encapsula um filtro e uma projeção para executar buscas encadeadas.
// Os terminais (Find, FindAll, FindPaged, Join) reutilizam o mesmo filtro/projeção.
type Query[T any] struct {
	repo   *Repository[T]
	filter *FilterBuilder
	proj   *ProjectBuilder
}

// Query cria um builder de busca a partir de um filtro (opcional) e uma projeção (opcional).
//
// Exemplo:
//
//	result, err := users.Query(monger.Filter().Eq("active", true), nil).
//	    FindAll(ctx, 100)
func (r *Repository[T]) Query(f *FilterBuilder, p *ProjectBuilder) *Query[T] {
	return &Query[T]{repo: r, filter: f, proj: p}
}

// Find busca um único documento usando o filtro/projeção da query.
// O filtro é obrigatório (mesma regra de Repository.Find).
func (q *Query[T]) Find(ctx context.Context) (*T, error) {
	return q.repo.Find(ctx, q.filter, q.proj)
}

// FindAll busca múltiplos documentos usando o filtro/projeção da query.
func (q *Query[T]) FindAll(ctx context.Context, limit int64) ([]T, error) {
	return q.repo.FindAll(ctx, q.filter, q.proj, limit)
}

// FindPaged busca paginado usando o filtro/projeção da query.
func (q *Query[T]) FindPaged(ctx context.Context, skip, limit int64, sort D) (*PagedResult[T], error) {
	return q.repo.FindPaged(ctx, q.filter, q.proj, skip, limit, sort)
}
