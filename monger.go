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
