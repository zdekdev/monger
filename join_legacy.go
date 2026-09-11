package monger

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
)

// --- JOIN LEGADO (deprecated) ---

// JoinCollection representa uma coleção a ser unida no Join.
//
// Deprecated: use Query.Join com JoinRef (criado por Ref). Será removido na v2.0.0.
type JoinCollection struct {
	Collection *mongo.Collection // Coleção do MongoDB
	Field      string            // Campo local a ser usado na junção (pode ser diferente do campo comum)
	Alias      string            // Alias para os campos dessa coleção no resultado (opcional)
}

// NewJoinCollection cria uma JoinCollection a partir de um Repository.
//
// Deprecated: use Ref com Query.Join. Será removido na v2.0.0.
func NewJoinCollection[T any](repo *Repository[T], field string, alias string) JoinCollection {
	return JoinCollection{
		Collection: repo.coll,
		Field:      field,
		Alias:      alias,
	}
}

// Join busca documentos em múltiplas coleções que compartilham um valor comum em um campo específico.
// Retorna um único documento (M) contendo a união de todos os campos encontrados.
//
// Parâmetros:
//   - ctx: contexto da operação
//   - commonValue: valor do campo comum a ser buscado (ex: um ID, CPF, email, etc.)
//   - collections: lista de JoinCollection contendo as coleções e configurações
//
// Exemplo de uso:
//
//	result, err := monger.Join(ctx, "12345678900",
//	    monger.NewJoinCollection(usersRepo, "cpf", "user"),
//	    monger.NewJoinCollection(ordersRepo, "customerCpf", "orders"),
//	    monger.NewJoinCollection(addressRepo, "ownerCpf", "address"),
//	)
//
// Deprecated: use Query.Join com JoinRef (criado por Ref). Será removido na v2.0.0.
func Join(ctx context.Context, commonValue any, collections ...JoinCollection) (*JoinResult, error) {
	if len(collections) == 0 {
		return nil, fmt.Errorf("pelo menos uma coleção é necessária")
	}

	result := M{}

	for _, jc := range collections {
		if jc.Collection == nil {
			return nil, fmt.Errorf("coleção não pode ser nil")
		}
		if jc.Field == "" {
			return nil, fmt.Errorf("field não pode ser vazio")
		}

		// Busca o documento na coleção
		var doc M
		err := jc.Collection.FindOne(ctx, M{jc.Field: commonValue}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				continue // Documento não encontrado, pula para a próxima coleção
			}
			return nil, fmt.Errorf("erro ao buscar na coleção: %w", err)
		}

		// Mescla os campos no resultado
		if jc.Alias != "" {
			// Com alias: agrupa os campos sob o alias
			result[jc.Alias] = doc
		} else {
			// Sem alias: mescla os campos diretamente no resultado
			for k, v := range doc {
				result[k] = v
			}
		}
	}

	if len(result) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return &JoinResult{Data: result}, nil
}

// JoinAll busca TODOS os documentos em cada coleção que compartilham o valor comum.
// Similar ao Join, mas retorna arrays quando há múltiplos documentos em uma coleção.
//
// Parâmetros:
//   - ctx: contexto da operação
//   - commonValue: valor do campo comum a ser buscado
//   - collections: lista de JoinCollection
//
// Exemplo de uso:
//
//	result, err := monger.JoinAll(ctx, "12345678900",
//	    monger.NewJoinCollection(usersRepo, "cpf", "user"),
//	    monger.NewJoinCollection(ordersRepo, "customerCpf", "orders"), // pode ter múltiplos pedidos
//	)
//
// Deprecated: use Query.Join com JoinRef (criado por Ref). Será removido na v2.0.0.
func JoinAll(ctx context.Context, commonValue any, collections ...JoinCollection) (*JoinResult, error) {
	if len(collections) == 0 {
		return nil, fmt.Errorf("pelo menos uma coleção é necessária")
	}

	result := M{}

	for _, jc := range collections {
		if jc.Collection == nil {
			return nil, fmt.Errorf("coleção não pode ser nil")
		}
		if jc.Field == "" {
			return nil, fmt.Errorf("field não pode ser vazio")
		}

		// Busca todos os documentos na coleção
		cursor, err := jc.Collection.Find(ctx, M{jc.Field: commonValue})
		if err != nil {
			return nil, fmt.Errorf("erro ao buscar na coleção: %w", err)
		}

		var docs []M
		if err := cursor.All(ctx, &docs); err != nil {
			cursor.Close(ctx)
			return nil, fmt.Errorf("erro ao decodificar documentos: %w", err)
		}
		cursor.Close(ctx)

		if len(docs) == 0 {
			continue
		}

		// Mescla os campos no resultado
		if jc.Alias != "" {
			if len(docs) == 1 {
				result[jc.Alias] = docs[0]
			} else {
				result[jc.Alias] = docs
			}
		} else {
			// Sem alias: mescla apenas o primeiro documento diretamente
			for k, v := range docs[0] {
				result[k] = v
			}
		}
	}

	if len(result) == 0 {
		return nil, mongo.ErrNoDocuments
	}

	return &JoinResult{Data: result}, nil
}
