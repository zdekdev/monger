package monger

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- JOIN (união de coleções) ---

// JoinResult encapsula o resultado da união de múltiplas coleções
type JoinResult struct {
	Data M `json:"data" bson:",inline"`
}

// JoinRef representa uma coleção a ser unida no Join encadeado (Query.Join).
type JoinRef struct {
	Collection   *mongo.Collection // Coleção do MongoDB
	ForeignField string            // Campo da coleção externa que casa com o campo local
	As           string            // Nome do campo no resultado (vazio => nome da coleção)
}

// Ref cria um JoinRef a partir de uma coleção (Repository), derivando o alias do
// nome da coleção no MongoDB. Use o campo As para sobrescrever o alias.
//
// Exemplo:
//
//	monger.Ref(ordersRepo, "customerCpf") // As = "orders"
//
// Para um alias customizado:
//
//	ref := monger.Ref(ordersRepo, "customerCpf")
//	ref.As = "pedidos"
func Ref[T any](repo *Repository[T], foreignField string) JoinRef {
	return JoinRef{
		Collection:   repo.coll,
		ForeignField: foreignField,
		As:           repo.coll.Name(),
	}
}

// Join executa a união de coleções usando o MESMO filtro/projeção da Query.
//
// Para cada documento base retornado pelo filtro, lê o valor de localField e busca,
// em cada coleção de refs, os documentos cujo ForeignField casa com esse valor.
//
// Comportamento do vínculo (por ref):
//   - 0 documentos  -> o campo As é omitido;
//   - 1 documento   -> vira objeto;
//   - >1 documentos -> vira array.
//
// Retorna uma lista de documentos mesclados (um por documento base), na mesma ordem
// dos documentos base. A projeção da Query é aplicada aos documentos base (garantindo
// o localField); os documentos unidos entram sob ref.As ou o nome da coleção.
//
// Exemplo:
//
//	res, err := users.
//	    Query(monger.Filter().Eq("cpf", "12345678900"), nil).
//	    Join(ctx, "cpf",
//	        monger.Ref(ordersRepo, "customerCpf"),
//	        monger.Ref(addressRepo, "ownerCpf"),
//	    )
func (q *Query[T]) Join(ctx context.Context, localField string, refs ...JoinRef) ([]M, error) {
	if q.repo == nil {
		return nil, fmt.Errorf("query sem repositório")
	}
	if localField == "" {
		return nil, fmt.Errorf("localField é obrigatório")
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("pelo menos uma coleção é necessária")
	}
	for _, ref := range refs {
		if ref.Collection == nil {
			return nil, fmt.Errorf("coleção não pode ser nil")
		}
		if ref.ForeignField == "" {
			return nil, fmt.Errorf("ForeignField não pode ser vazio")
		}
	}

	filter := M{}
	if q.filter != nil {
		filter = q.filter.Build()
	}

	opts := options.Find()
	if q.proj != nil {
		opts.SetProjection(joinProjection(q.proj.Build(), localField))
	}

	cursor, err := q.repo.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var bases []M
	if err := cursor.All(ctx, &bases); err != nil {
		return nil, err
	}

	results := make([]M, 0, len(bases))
	for _, base := range bases {
		merged := M{}
		for k, v := range base {
			merged[k] = v
		}

		value, ok := base[localField]
		if !ok {
			results = append(results, merged)
			continue
		}

		for _, ref := range refs {
			as := ref.As
			if as == "" {
				as = ref.Collection.Name()
			}

			cur, err := ref.Collection.Find(ctx, M{ref.ForeignField: value})
			if err != nil {
				return nil, fmt.Errorf("erro ao buscar na coleção %s: %w", as, err)
			}

			var docs []M
			if err := cur.All(ctx, &docs); err != nil {
				cur.Close(ctx)
				return nil, fmt.Errorf("erro ao decodificar a coleção %s: %w", as, err)
			}
			cur.Close(ctx)

			switch len(docs) {
			case 0:
				// sem correspondência: omite o campo
			case 1:
				merged[as] = docs[0]
			default:
				merged[as] = docs
			}
		}

		results = append(results, merged)
	}

	return results, nil
}

// joinProjection garante que localField esteja presente na projeção para que o
// Join consiga ler o valor de junção dos documentos base.
func joinProjection(p M, localField string) M {
	out := M{}
	inclusive := false
	for _, v := range p {
		if isProjectionOn(v) {
			inclusive = true
			break
		}
	}
	for k, v := range p {
		if !inclusive && k == localField {
			continue // remove a exclusão do campo de junção
		}
		out[k] = v
	}
	if inclusive {
		out[localField] = 1
	}
	return out
}

// isProjectionOn indica se um valor de projeção inclui o campo (1/true).
func isProjectionOn(v any) bool {
	switch n := v.(type) {
	case int:
		return n != 0
	case int32:
		return n != 0
	case int64:
		return n != 0
	case bool:
		return n
	}
	return false
}

// LookupConfig configura um estágio $lookup em JoinWithLookup.
type LookupConfig struct {
	From         string // Nome da coleção externa
	ForeignField string // Campo na coleção externa (chave de relacionamento na coleção externa)
	As           string // Nome do campo no resultado
}

// JoinWithLookup usa agregação $lookup do MongoDB para fazer join server-side.
// Mais eficiente para grandes volumes de dados pois o join é feito no servidor.
//
// Parâmetros:
//   - ctx: contexto da operação
//   - baseCollection: coleção base (de onde a agregação começa)
//   - localField: campo na coleção base
//   - localValue: valor a ser buscado na coleção base
//   - lookups: configurações de lookup para cada coleção a ser unida
//
// Exemplo de uso:
//
//	result, err := monger.JoinWithLookup(ctx, usersRepo.Collection(), "cpf", "12345678900",
//	    monger.LookupConfig{From: "orders", ForeignField: "customerCpf", As: "orders"},
//	    monger.LookupConfig{From: "addresses", ForeignField: "ownerCpf", As: "address"},
//	)
func JoinWithLookup(ctx context.Context, baseCollection *mongo.Collection, localField string, localValue any, lookups ...LookupConfig) (*JoinResult, error) {
	if baseCollection == nil {
		return nil, fmt.Errorf("baseCollection não pode ser nil")
	}

	// Pipeline de agregação
	pipeline := []M{
		{"$match": M{localField: localValue}},
	}

	// Adiciona os lookups
	for _, lc := range lookups {
		if lc.From == "" || lc.ForeignField == "" || lc.As == "" {
			return nil, fmt.Errorf("LookupConfig inválido: From, ForeignField e As são obrigatórios")
		}

		// Usa $lookup com pipeline para permitir remover o campo de junção (ForeignField)
		// e evitar repetir o valor que já existe no documento base (ex.: cpf).
		lookupPipeline := []M{
			{
				"$match": M{
					"$expr": M{
						"$eq": []any{"$" + lc.ForeignField, "$$localValue"},
					},
				},
			},
		}
		// Remove a chave usada no relacionamento do resultado do lookup.
		lookupPipeline = append(lookupPipeline, M{"$project": M{lc.ForeignField: 0}})

		pipeline = append(pipeline, M{
			"$lookup": M{
				"from":     lc.From,
				"let":      M{"localValue": "$" + localField},
				"pipeline": lookupPipeline,
				"as":       lc.As,
			},
		})
	}

	// Limita a um documento
	pipeline = append(pipeline, M{"$limit": 1})

	cursor, err := baseCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("erro na agregação: %w", err)
	}
	defer cursor.Close(ctx)

	var results []M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resultado: %w", err)
	}

	if len(results) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return &JoinResult{Data: results[0]}, nil
}
