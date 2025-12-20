package monger

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type M = bson.M
type D = bson.D

// --- RESULTADOS ---
type PagedResult[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
}

// --- FILTER BUILDER (Onde/Filtro) ---
type FilterBuilder struct{ f M }

func Filter() *FilterBuilder                                           { return &FilterBuilder{f: M{}} }
func (b *FilterBuilder) Eq(field string, val any) *FilterBuilder       { b.f[field] = val; return b }
func (b *FilterBuilder) Gt(field string, val any) *FilterBuilder       { b.f[field] = M{"$gt": val}; return b }
func (b *FilterBuilder) In(field string, vals any) *FilterBuilder      { b.f[field] = M{"$in": vals}; return b }
func (b *FilterBuilder) And(bs ...*FilterBuilder) *FilterBuilder {
	fs := []M{}
	for _, s := range bs { fs = append(fs, s.Build()) }
	b.f["$and"] = fs
	return b
}
func (b *FilterBuilder) Build() M { return b.f }

// --- PROJECT BUILDER (Seleção de Campos) ---
type ProjectBuilder struct{ p M }

func Select(fields ...string) *ProjectBuilder {
	m := M{}
	for _, f := range fields { m[f] = 1 }
	return &ProjectBuilder{p: m}
}

func Exclude(fields ...string) *ProjectBuilder {
	m := M{}
	for _, f := range fields { m[f] = 0 }
	return &ProjectBuilder{p: m}
}

func (b *ProjectBuilder) Build() M { return b.p }

// --- REPOSITÓRIO ---
type Repository[T any] struct {
	coll *mongo.Collection
}

func New[T any](db *mongo.Database, collectionName string) *Repository[T] {
	return &Repository[T]{coll: db.Collection(collectionName)}
}

// Auxiliar para extrair filtros e projeções de forma segura
func (r *Repository[T]) getOpts(f *FilterBuilder, p *ProjectBuilder) (M, *options.FindOptions) {
	filter := M{}
	if f != nil { filter = f.Build() }
	opts := options.Find()
	if p != nil { opts.SetProjection(p.Build()) }
	return filter, opts
}

func (r *Repository[T]) InsertOne(ctx context.Context, model *T) (string, error) {
	res, err := r.coll.InsertOne(ctx, model)
	if err != nil { return "", err }
	return res.InsertedID.(primitive.ObjectID).Hex(), nil
}

func (r *Repository[T]) FindByID(ctx context.Context, id string, p *ProjectBuilder) (*T, error) {
	oid, _ := primitive.ObjectIDFromHex(id)
	opts := options.FindOne()
	if p != nil { opts.SetProjection(p.Build()) }
	var res T
	err := r.coll.FindOne(ctx, M{"_id": oid}, opts).Decode(&res)
	return &res, err
}

func (r *Repository[T]) Find(ctx context.Context, f *FilterBuilder, p *ProjectBuilder) ([]T, error) {
	filter, opts := r.getOpts(f, p)
	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil { return nil, err }
	defer cursor.Close(ctx)
	var results []T
	return results, cursor.All(ctx, &results)
}

func (r *Repository[T]) FindPaged(ctx context.Context, f *FilterBuilder, p *ProjectBuilder, skip, limit int64, sort D) (*PagedResult[T], error) {
	filter, opts := r.getOpts(f, p)
	total, _ := r.coll.CountDocuments(ctx, filter)
	
	opts.SetLimit(limit).SetSkip(skip).SetSort(sort)
	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil { return nil, err }
	defer cursor.Close(ctx)

	var data []T
	err = cursor.All(ctx, &data)
	return &PagedResult[T]{Data: data, Total: total}, err
}

func (r *Repository[T]) UpdateByID(ctx context.Context, id string, model *T) error {
	oid, _ := primitive.ObjectIDFromHex(id)
	_, err := r.coll.ReplaceOne(ctx, M{"_id": oid}, model)
	return err
}

func (r *Repository[T]) DeleteByID(ctx context.Context, id string) error {
	oid, _ := primitive.ObjectIDFromHex(id)
	_, err := r.coll.DeleteOne(ctx, M{"_id": oid})
	return err
}