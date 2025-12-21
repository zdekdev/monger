# 📦 Monger

Monger é um wrapper leve (e genérico) em Go para facilitar operações comuns com MongoDB usando o driver oficial (`go.mongodb.org/mongo-driver`).

Ele fornece:

- Um `Repository[T]` genérico com operações CRUD e paginação.
- Um `FilterBuilder` para montar filtros BSON de forma fluente (sem a verbosidade de `bson.M` direto).
- Um `ProjectBuilder` para projeção de campos (equivalente a `SELECT`/`projection`).

> O Monger não substitui o driver oficial: ele organiza e reduz boilerplate para casos comuns.

---

## Requisitos

- Go 1.18+ (para generics). O `go.mod` do projeto está em Go `1.25.0`.
- MongoDB acessível e o driver oficial do MongoDB (vem como dependência transitiva).

---

## Instalação

```bash
go get github.com/zdekdev/monger
```

Import:

```go
import "github.com/zdekdev/monger"
```

---

## Quickstart

Exemplo completo com conexão, criação de repositório e operações básicas.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zdekdev/monger"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Age       int                `bson:"age" json:"age"`
	Active    bool               `bson:"active" json:"active"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

func main() {
	ctx := context.Background()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

	db := client.Database("app")
	users := monger.New[User](db, "users")

	// Insert
	u := User{Name: "Ana", Age: 29, Active: true, CreatedAt: time.Now()}
	id, err := users.InsertOne(ctx, &u)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("inserted id:", id)

	// Find by id (com projeção opcional)
	found, err := users.FindByID(ctx, id, monger.Select("name", "age"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("found: %+v\n", found)

	// Find com filtro
	list, err := users.Find(ctx,
		monger.Filter().
			Eq("active", true).
			Gte("age", 18),
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("total active adults:", len(list))
}
```

---

## Conceitos principais

### Tipos utilitários (`M` e `D`)

O pacote expõe aliases para facilitar a construção de BSON quando necessário:

- `type M = bson.M` (map)
- `type D = bson.D` (slice ordenado)

Isso é útil principalmente para `sort` e para casos onde você quer usar diretamente operadores BSON do driver.

---

## FilterBuilder (montagem de filtros)

Crie filtros com `monger.Filter()` e encadeie comparadores.

### Comparadores

- `Eq(field, val)` / `Equal(field, val)` → `{field: val}`
- `Ne(field, val)` / `NotEqual(field, val)` → `{field: {$ne: val}}`
- `Gt(field, val)` / `GreaterThan(field, val)` → `{field: {$gt: val}}`
- `Gte(field, val)` / `GreaterThanOrEqual(field, val)` → `{field: {$gte: val}}`
- `Lt(field, val)` / `LessThan(field, val)` → `{field: {$lt: val}}`
- `Lte(field, val)` / `LessThanOrEqual(field, val)` → `{field: {$lte: val}}`
- `In(field, vals)` / `InValues(field, vals)` → `{field: {$in: vals}}`

> Observação: `vals` deve ser algo que o driver aceite para `$in` (ex.: `[]string`, `[]int`, etc).

### Operadores lógicos (`And` / `Or`)

Você pode compor filtros:

```go
f := monger.Filter().And(
	monger.Filter().Eq("active", true),
	monger.Filter().Or(
		monger.Filter().Gte("age", 18),
		monger.Filter().Eq("role", "admin"),
	),
)
```

### Build

`Build()` retorna um `monger.M` (alias de `bson.M`) pronto para uso no driver.

---

## ProjectBuilder (projeção de campos)

Use para controlar quais campos voltam do banco:

- `monger.Select("a", "b")` → `{a: 1, b: 1}`
- `monger.Exclude("a", "b")` → `{a: 0, b: 0}`

Exemplo:

```go
u, err := users.FindByID(ctx, id, monger.Select("name", "age"))
```

---

## Repository[T]

`Repository[T]` encapsula uma `*mongo.Collection` e expõe métodos comuns.

### Criando um repositório

```go
users := monger.New[User](db, "users")
```

### InsertOne

Insere um documento e retorna o `_id` em formato hex string (ObjectID):

```go
id, err := users.InsertOne(ctx, &User{Name: "João"})
```

### FindByID

Busca um documento pelo `_id` (hex). Aceita projeção opcional:

```go
u, err := users.FindByID(ctx, id, nil)
u2, err := users.FindByID(ctx, id, monger.Select("name"))
```

### Find

Busca múltiplos documentos com filtro/projeção:

```go
list, err := users.Find(ctx,
	monger.Filter().Eq("active", true),
	monger.Exclude("passwordHash"),
)
```

### FindAll

Atalho para retornar todos os documentos:

```go
list, err := users.FindAll(ctx)
```

### Count

Conta documentos que satisfazem um filtro:

```go
total, err := users.Count(ctx, monger.Filter().Eq("active", true))
```

### Exists

Retorna `true` se existir ao menos um documento que satisfaça o filtro:

```go
ok, err := users.Exists(ctx, monger.Filter().Eq("email", "a@b.com"))
```

### FindPaged (paginação + sort)

Retorna `PagedResult[T]` com `Data` e `Total` (total de documentos do filtro, sem paginação).

```go
res, err := users.FindPaged(
	ctx,
	monger.Filter().Eq("active", true),
	monger.Select("name", "createdAt"),
	0,  // skip
	10, // limit
	monger.D{{Key: "createdAt", Value: -1}}, // sort desc
)
if err != nil {
	// handle
}

fmt.Println("total:", res.Total)
fmt.Println("page size:", len(res.Data))
```

### UpdateByID (update parcial)

Atualiza parcialmente o documento.

**1) Via struct (padrão):** usa `$set` apenas com campos **não-zerados**.

```go
err := users.UpdateByID(ctx, id, &User{Name: "Novo Nome"})
```

No exemplo acima, somente o campo `Name` será atualizado; os demais campos do documento permanecem como estão.

**2) Valores “zerados” (0, "", false):** em Go não dá para distinguir “campo não informado” de “campo informado com zero” usando apenas um struct comum.
Para manter o código enxuto e ainda permitir atualizar valores zerados, use um *patch struct* com campos ponteiro.

Exemplo:

```go
type UserPatch struct {
	Name   *string `bson:"name"`
	Age    *int    `bson:"age"`
	Active *bool   `bson:"active"`
}

// Você pode declarar só os campos que pretende atualizar.
// Ex.: type UserPatch struct { Active *bool `bson:"active"` }

// atualiza explicitamente para false
err := users.UpdateByID(ctx, id, &UserPatch{Active: monger.Value(false)})

// atualiza explicitamente para 0
err = users.UpdateByID(ctx, id, &UserPatch{Age: monger.Value(0)})

// atualiza explicitamente para string vazia
err = users.UpdateByID(ctx, id, &UserPatch{Name: monger.Value("")})
```

> Observação: o campo `_id` é ignorado caso seja enviado no update.

### DeleteByID

Remove um documento pelo `_id`:

```go
err := users.DeleteByID(ctx, id)
```

---

## Dicas de uso

- Use `context.Context` com timeout/cancelamento (principalmente em produção).
- Para ordenação, prefira `monger.D` (alias de `bson.D`) pois preserva ordem dos campos.
- Para filtros complexos com operadores que não estão no builder (ex.: `$regex`, `$elemMatch`), você pode misturar com `monger.M` diretamente via `Build()` ou criar um `bson.M` manual.

---

## Licença

Veja [LICENSE](LICENSE).