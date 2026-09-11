package monger

import "go.mongodb.org/mongo-driver/bson"

// Aliases para facilitar o uso interno e externo
type M = bson.M
type D = bson.D

// Value retorna um ponteiro para o valor informado.
// Útil para "patch structs" (campos ponteiro) em updates parciais, inclusive com valores zerados (0, "", false).
func Value[T any](v T) *T { return &v }

// PagedResult encapsula os dados retornados e o total para paginação
type PagedResult[T any] struct {
	Data  []T   `json:"data"`
	Total int64 `json:"total"`
}
