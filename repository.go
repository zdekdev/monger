package monger

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// --- REPOSITORY ---
type Repository[T any] struct {
	coll *mongo.Collection
}

func New[T any](db *mongo.Database, collectionName string) *Repository[T] {
	return &Repository[T]{coll: db.Collection(collectionName)}
}

// Collection retorna a coleção MongoDB subjacente do Repository
func (r *Repository[T]) Collection() *mongo.Collection {
	return r.coll
}

// InsertOne insere um documento e retorna o ID hex
func (r *Repository[T]) InsertOne(ctx context.Context, model *T) (string, error) {
	res, err := r.coll.InsertOne(ctx, model)
	if err != nil {
		return "", err
	}
	oid, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", fmt.Errorf("erro ao converter ID inserido")
	}
	return oid.Hex(), nil
}

// InsertOneAndUpdate realiza um upsert: se o documento já existir (baseado no filtro), atualiza apenas os campos diferentes;
// se não existir, insere o documento completo.
//
// Parâmetros:
//   - ctx: contexto da operação
//   - filter: filtro para identificar o documento (obrigatório, use FilterBuilder com um campo único como email, cpf, etc.)
//   - model: documento a ser inserido ou usado para atualização
//
// Retorna o ID do documento (inserido ou existente) e um booleano indicando se foi uma inserção (true) ou atualização (false).
//
// Nota: Para inserir novos documentos, use InsertOne. Para atualizar por _id, use UpdateByID.
// Use InsertOneAndUpdate apenas para upsert por campos únicos (ex: email, cpf, sku).
func (r *Repository[T]) InsertOneAndUpdate(ctx context.Context, filter *FilterBuilder, model *T) (string, bool, error) {
	if filter == nil {
		return "", false, fmt.Errorf("filter é obrigatório")
	}
	if model == nil {
		return "", false, fmt.Errorf("model não pode ser nil")
	}

	// Monta o filtro
	f := filter.Build()
	if len(f) == 0 {
		return "", false, fmt.Errorf("filter não pode ser vazio")
	}

	// Constrói o documento de update
	doc, err := buildPartialUpdate(model, false)
	if err != nil {
		return "", false, err
	}

	opts := options.Update().SetUpsert(true)
	res, err := r.coll.UpdateOne(ctx, f, M{"$set": doc}, opts)
	if err != nil {
		return "", false, err
	}

	// Determina o ID retornado
	var id string
	isInsert := res.UpsertedCount > 0

	if isInsert {
		// Documento foi inserido
		if oid, ok := res.UpsertedID.(primitive.ObjectID); ok {
			id = oid.Hex()
		} else {
			return "", false, fmt.Errorf("erro ao converter ID do upsert")
		}
	} else {
		// Documento foi atualizado - busca o ID existente
		if oid, ok := f["_id"].(primitive.ObjectID); ok {
			id = oid.Hex()
		} else {
			// Busca o documento para obter o ID
			var existing M
			err := r.coll.FindOne(ctx, f, options.FindOne().SetProjection(M{"_id": 1})).Decode(&existing)
			if err != nil {
				return "", false, fmt.Errorf("erro ao buscar ID do documento atualizado: %w", err)
			}
			if oid, ok := existing["_id"].(primitive.ObjectID); ok {
				id = oid.Hex()
			}
		}
	}

	return id, isInsert, nil
}

// Find busca um único documento com filtro e projeção.
// Ideal para buscas por campos únicos como _id, cpf, email, etc.
// O filtro é obrigatório para evitar retornar documentos aleatórios.
//
// Exemplo de uso:
//
//	// Buscar por ID
//	user, err := users.Find(ctx, monger.Filter().Eq("_id", oid), nil)
//
//	// Buscar por CPF
//	user, err := users.Find(ctx, monger.Filter().Eq("cpf", "12345678900"), nil)
//
//	// Buscar por email com projeção
//	user, err := users.Find(ctx, monger.Filter().Eq("email", "ana@email.com"), monger.Select("name", "email"))
func (r *Repository[T]) Find(ctx context.Context, f *FilterBuilder, p *ProjectBuilder) (*T, error) {
	if f == nil {
		return nil, fmt.Errorf("filtro é obrigatório para Find; use FindAll para buscar múltiplos documentos")
	}
	filter := f.Build()
	opts := options.FindOne()
	if p != nil {
		opts.SetProjection(p.Build())
	}
	var res T
	err := r.coll.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// FindAll busca múltiplos documentos com filtro e projeção.
// O filtro usa busca "fuzzy" (regex case-insensitive) para campos string,
// permitindo encontrar documentos mesmo com erros de digitação ou nomes parciais.
//
// Parâmetros:
//   - ctx: contexto da operação
//   - f: filtro (opcional, se nil retorna todos os documentos)
//   - p: projeção (opcional)
//   - limit: limite de resultados (use 0 para sem limite - use com cuidado!)
//
// Exemplo de uso:
//
//	// Buscar clientes por nome (fuzzy match)
//	clients, err := users.FindAll(ctx, monger.Filter().Eq("name", "João"), nil, 100)
//	// Retorna: "João Silva", "João Pedro", "Maria João", etc.
//
//	// Buscar todos os ativos com limite
//	clients, err := users.FindAll(ctx, monger.Filter().Eq("active", true), nil, 50)
//
//	// Buscar todos sem limite (cuidado com performance!)
//	allClients, err := users.FindAll(ctx, nil, nil, 0)
func (r *Repository[T]) FindAll(ctx context.Context, f *FilterBuilder, p *ProjectBuilder, limit int64) ([]T, error) {
	filter := M{}
	if f != nil {
		filter = convertToFuzzyFilter(f.Build())
	}

	opts := options.Find()
	if p != nil {
		opts.SetProjection(p.Build())
	}
	if limit > 0 {
		opts.SetLimit(limit)
	}

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []T
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// Count conta documentos baseados em um filtro
func (r *Repository[T]) Count(ctx context.Context, f *FilterBuilder) (int64, error) {
	filter := M{}
	if f != nil {
		filter = f.Build()
	}
	return r.coll.CountDocuments(ctx, filter)
}

// Exists verifica se existe ao menos um documento que satisfaça o filtro
func (r *Repository[T]) Exists(ctx context.Context, f *FilterBuilder) (bool, error) {
	filter := M{}
	if f != nil {
		filter = f.Build()
	}
	count, err := r.coll.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	return count > 0, err
}

// FindPaged realiza busca com paginação, ordenação e projeção.
// Se o filtro for nil, retorna todos os documentos respeitando a paginação.
//
// Parâmetros:
//   - ctx: contexto da operação
//   - f: filtro (opcional, se nil retorna todos os documentos)
//   - p: projeção (opcional)
//   - skip: número de documentos a pular
//   - limit: número máximo de documentos a retornar
//   - sort: ordenação (use monger.D para preservar ordem)
//
// Exemplo de uso:
//
//	// Listar todos os usuários paginados
//	res, err := users.FindPaged(ctx, nil, nil, 0, 10, monger.D{{Key: "name", Value: 1}})
//
//	// Listar usuários ativos paginados
//	res, err := users.FindPaged(ctx, monger.Filter().Eq("active", true), nil, 0, 10, nil)
func (r *Repository[T]) FindPaged(ctx context.Context, f *FilterBuilder, p *ProjectBuilder, skip, limit int64, sort D) (*PagedResult[T], error) {
	filter := M{}
	if f != nil {
		filter = f.Build()
	}

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	opts := options.Find()
	if p != nil {
		opts.SetProjection(p.Build())
	}
	opts.SetLimit(limit).SetSkip(skip)
	if sort != nil {
		opts.SetSort(sort)
	}

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var data []T
	if err := cursor.All(ctx, &data); err != nil {
		return nil, err
	}

	return &PagedResult[T]{
		Data:  data,
		Total: total,
	}, nil
}

// DeleteByID remove um documento por ID
func (r *Repository[T]) DeleteByID(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = r.coll.DeleteOne(ctx, M{"_id": oid})
	return err
}
