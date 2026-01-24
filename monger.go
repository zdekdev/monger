package monger

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Aliases para facilitar o uso interno e externo
type M = bson.M
type D = bson.D

// Ptr retorna um ponteiro para o valor informado.
// Útil para "patch structs" (campos ponteiro) em updates parciais, inclusive com valores zerados (0, "", false).
func Value[T any](v T) *T { return &v }

// PagedResult encapsula os dados retornados e o total para paginação
type PagedResult[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
}

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

// --- REPOSITORY ---
type Repository[T any] struct {
	coll *mongo.Collection
}

func New[T any](db *mongo.Database, collectionName string) *Repository[T] {
	return &Repository[T]{coll: db.Collection(collectionName)}
}

// getOpts é um auxiliar interno para preparar filtros e projeções
func (r *Repository[T]) getOpts(f *FilterBuilder, p *ProjectBuilder) (M, *options.FindOptions) {
	filter := M{}
	if f != nil {
		filter = f.Build()
	}
	opts := options.Find()
	if p != nil {
		opts.SetProjection(p.Build())
	}
	return filter, opts
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
	doc, err := buildPartialUpdate(model)
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

// FindByID busca um único documento por ID com projeção opcional
func (r *Repository[T]) FindByID(ctx context.Context, id string, p *ProjectBuilder) (*T, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("id inválido: %w", err)
	}
	opts := options.FindOne()
	if p != nil {
		opts.SetProjection(p.Build())
	}
	var res T
	err = r.coll.FindOne(ctx, M{"_id": oid}, opts).Decode(&res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Find busca múltiplos documentos com filtro e projeção
func (r *Repository[T]) Find(ctx context.Context, f *FilterBuilder, p *ProjectBuilder) ([]T, error) {
	filter, opts := r.getOpts(f, p)
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

// FindAll retorna todos os documentos da coleção
func (r *Repository[T]) FindAll(ctx context.Context) ([]T, error) {
	return r.Find(ctx, nil, nil)
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

// FindPaged realiza busca com paginação, ordenação e projeção
func (r *Repository[T]) FindPaged(ctx context.Context, f *FilterBuilder, p *ProjectBuilder, skip, limit int64, sort D) (*PagedResult[T], error) {
	filter, opts := r.getOpts(f, p)
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	opts.SetLimit(limit).SetSkip(skip).SetSort(sort)
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

func parseBsonTag(tag string) (name string, inline bool) {
	if tag == "" {
		return "", false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, opt := range parts[1:] {
		if opt == "inline" {
			inline = true
			break
		}
	}
	return name, inline
}

func buildPartialUpdate(doc any) (M, error) {
	v := reflect.ValueOf(doc)
	if !v.IsValid() {
		return M{}, nil
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return M{}, nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("modelo precisa ser struct ou ponteiro para struct")
	}

	t := v.Type()
	update := M{}
	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // não-exportado
			continue
		}

		tag, inline := parseBsonTag(sf.Tag.Get("bson"))
		if tag == "-" {
			continue
		}

		fv := v.Field(i)
		if inline || (sf.Anonymous && (fv.Kind() == reflect.Struct || (fv.Kind() == reflect.Pointer && fv.Elem().Kind() == reflect.Struct))) {
			sub, err := buildPartialUpdate(fv.Interface())
			if err != nil {
				return nil, err
			}
			for k, val := range sub {
				update[k] = val
			}
			continue
		}

		name := tag
		if name == "" {
			name = sf.Name
		}
		if name == "_id" {
			continue
		}

		// Regra: só inclui campos "não-zerados".
		// Para conseguir setar valores zerados (0, "", false), use ponteiros (*int, *string, *bool) no seu model.
		if fv.Kind() == reflect.Pointer || fv.Kind() == reflect.Interface {
			if fv.IsNil() {
				continue
			}
			if fv.Kind() == reflect.Pointer {
				update[name] = fv.Elem().Interface()
				continue
			}
			update[name] = fv.Interface()
			continue
		}
		if fv.IsZero() {
			continue
		}
		update[name] = fv.Interface()
	}
	return update, nil
}

// UpdateByID faz update parcial do documento (UpdateOne + $set).
//
// Por padrão, só inclui campos não-zerados do struct.
// Para setar valores zerados (0, "", false), use um "patch struct" com campos ponteiro (*int, *string, *bool, etc.).
func (r *Repository[T]) UpdateByID(ctx context.Context, id string, update any) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	if update == nil {
		return fmt.Errorf("update não pode ser nil")
	}

	doc, err := buildPartialUpdate(update)
	if err != nil {
		return err
	}
	if len(doc) == 0 {
		return fmt.Errorf("nenhum campo para atualizar")
	}
	delete(doc, "_id")

	_, err = r.coll.UpdateOne(ctx, M{"_id": oid}, M{"$set": doc})
	return err
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

// --- JOIN (união de coleções) ---

// JoinResult encapsula o resultado da união de múltiplas coleções
type JoinResult struct {
	Data M `json:"data" bson:",inline"`
}

// JoinCollection representa uma coleção a ser unida no Join
type JoinCollection struct {
	Collection *mongo.Collection // Coleção do MongoDB
	Field      string            // Campo local a ser usado na junção (pode ser diferente do campo comum)
	Alias      string            // Alias para os campos dessa coleção no resultado (opcional)
}

// NewJoinCollection cria uma JoinCollection a partir de um Repository
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
type LookupConfig struct {
	From         string // Nome da coleção externa
	ForeignField string // Campo na coleção externa (chave de relacionamento na coleção externa)
	As           string // Nome do campo no resultado
}

// Collection retorna a coleção MongoDB subjacente do Repository
func (r *Repository[T]) Collection() *mongo.Collection {
	return r.coll
}

// JoinWithLookup executa uma agregação com $lookup para unir coleções no servidor
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
