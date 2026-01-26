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

	// Converter string ID para ObjectID
	oid, _ := primitive.ObjectIDFromHex(id)

	// Find por ID (com projeção opcional)
	found, err := users.Find(ctx, monger.Filter().Eq("_id", oid), monger.Select("name", "age"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("found: %+v\n", found)

	// FindAll com filtro (busca fuzzy)
	list, err := users.FindAll(ctx,
		monger.Filter().
			Eq("active", true).
			Gte("age", 18),
		nil,
		100, // limite de resultados
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

### InsertOneAndUpdate (Upsert)

Realiza um **upsert**: se o documento já existir (baseado no filtro), atualiza apenas os campos diferentes; se não existir, insere o documento completo.

Retorna:
- `id`: o ID do documento (inserido ou existente)
- `isInsert`: `true` se foi uma inserção, `false` se foi uma atualização
- `err`: erro, se houver

> **Quando usar cada função:**
> - Para **inserir** novos documentos: use `InsertOne`.
> - Para **atualizar** por `_id`: use `UpdateByID`.
> - Para **upsert** por campo único (ex: email, cpf, sku): use `InsertOneAndUpdate`.

**Exemplo 1: Upsert por email**

```go
// Busca por email único e insere/atualiza
user := User{Name: "Ana", Email: "ana@email.com", Age: 30}

id, isInsert, err := users.InsertOneAndUpdate(ctx,
    monger.Filter().Eq("email", "ana@email.com"),
    &user,
)
if err != nil {
    log.Fatal(err)
}

if isInsert {
    fmt.Println("Documento inserido com ID:", id)
} else {
    fmt.Println("Documento atualizado com ID:", id)
}
```

**Exemplo 2: Sincronização de dados externos**

```go
// Ideal para sincronizar dados de APIs externas
// Se o produto já existir (pelo SKU), atualiza o preço e estoque;
// senão, insere o produto completo
product := Product{SKU: "ABC123", Name: "Notebook", Price: 2999.90, Stock: 50}

id, isInsert, err := products.InsertOneAndUpdate(ctx,
    monger.Filter().Eq("sku", "ABC123"),
    &product,
)
```

> **Nota:** O filtro é **obrigatório** e deve usar um campo único (ex: `email`, `cpf`, `sku`).
> Apenas campos não-zerados são atualizados (mesma regra do `UpdateByID`). Para atualizar valores zerados (`0`, `""`, `false`), use um *patch struct* com campos ponteiro.

### Find

Busca um único documento com filtro. Ideal para buscas por campos únicos como `_id`, `cpf`, `email`, etc.
O filtro é **obrigatório** para evitar retornar documentos aleatórios.

```go
// Buscar por ID (converta a string para ObjectID primeiro)
oid, _ := primitive.ObjectIDFromHex(id)
u, err := users.Find(ctx, monger.Filter().Eq("_id", oid), nil)

// Buscar por CPF
u, err = users.Find(ctx, monger.Filter().Eq("cpf", "12345678900"), nil)

// Buscar por email com projeção
u, err = users.Find(ctx, monger.Filter().Eq("email", "ana@email.com"), monger.Select("name", "email"))
```

### FindAll

Busca múltiplos documentos com filtro e projeção. Usa busca **fuzzy** (regex case-insensitive) para campos string, permitindo encontrar documentos mesmo com erros de digitação ou nomes parciais.

```go
// Buscar clientes por nome (fuzzy match)
// Retorna: "João Silva", "João Pedro", "Maria João", etc.
clients, err := users.FindAll(ctx, monger.Filter().Eq("name", "João"), nil, 100)

// Buscar todos os ativos com limite
clients, err = users.FindAll(ctx, monger.Filter().Eq("active", true), nil, 50)

// Buscar todos sem filtro (com limite para segurança)
allClients, err := users.FindAll(ctx, nil, nil, 1000)

// Buscar todos sem limite (cuidado com performance em grandes coleções!)
allClients, err = users.FindAll(ctx, nil, nil, 0)
```

**Parâmetros:**
- `ctx`: contexto da operação
- `f`: filtro (opcional, se `nil` retorna todos os documentos)
- `p`: projeção (opcional)
- `limit`: limite de resultados (use `0` para sem limite - **use com cuidado!**)

> **Importante:** Para buscas em grandes coleções, sempre defina um limite razoável para evitar sobrecarga do servidor.
> A busca fuzzy só é aplicada em campos string; campos não-string (como `bool`, `int`, `ObjectID`) usam igualdade exata.

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
Se o filtro for `nil`, retorna todos os documentos respeitando a paginação.

```go
// Com filtro
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

// Sem filtro (lista todos os documentos paginados)
res, err = users.FindPaged(
	ctx,
	nil, // sem filtro - retorna todos
	nil,
	0,  // skip
	20, // limit
	monger.D{{Key: "name", Value: 1}}, // sort asc por nome
)
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

## Join (União de Coleções)

O Monger oferece funções para “juntar” dados de múltiplas coleções usando um **valor em comum** (por exemplo: `cpf`, `email`, `userId`).

Importante: `Join`/`JoinAll` fazem buscas **coleção por coleção** (várias consultas). Para volumes grandes, ou quando você quer que o Mongo faça a união no servidor, prefira `JoinWithLookup`.

### Receita rápida (como pensar)

1) Escolha o `commonValue` (o valor que vai servir de chave). Ex.: `"12345678900"`.

2) Para cada coleção, diga qual **campo** guarda esse valor.

3) Use `alias` (recomendado) para evitar colisão de campos e deixar o resultado organizado.

### JoinCollection

Representa uma coleção a ser unida. Use `NewJoinCollection` para criar a partir de um Repository:

```go
jc := monger.NewJoinCollection(usersRepo, "cpf", "user")
```

Parâmetros:
- `repo`: o Repository da coleção
- `field`: campo a ser usado na junção (ex: `"cpf"`, `"_id"`, `"email"`)
- `alias`: nome do campo no resultado.
	- Se **não vazio**: o documento daquela coleção fica aninhado em `result.Data[alias]`.
	- Se **vazio**: os campos são mesclados no nível raiz do resultado (se houver chaves iguais, a última coleção pode sobrescrever valores).

### Join

Busca um documento em cada coleção que contenha o valor comum e retorna um único documento mesclado:

```go
// Exemplo: buscar dados de um usuário em múltiplas coleções pelo CPF
result, err := monger.Join(ctx, "12345678900",
    monger.NewJoinCollection(usersRepo, "cpf", "user"),
    monger.NewJoinCollection(addressRepo, "ownerCpf", "address"),
    monger.NewJoinCollection(profileRepo, "documentCpf", "profile"),
)
if err != nil {
	log.Fatal(err)
}

// result.Data contém:
// {
//   "user": { "name": "Ana", "cpf": "12345678900", ... },
//   "address": { "street": "Rua X", "ownerCpf": "12345678900", ... },
//   "profile": { "bio": "...", "documentCpf": "12345678900", ... }
// }
fmt.Printf("%+v\n", result.Data)
```

Comportamento importante:

- Se uma coleção não tiver documento com o valor, ela é ignorada.
- Se nenhuma coleção retornar dados, o erro é `mongo.ErrNoDocuments`.
- O retorno é `*monger.JoinResult` e os dados ficam em `result.Data` (um `monger.M`, alias de `bson.M`).

Se o `alias` for vazio, os campos são mesclados diretamente no resultado:

```go
result, err := monger.Join(ctx, "12345678900",
    monger.NewJoinCollection(usersRepo, "cpf", ""),      // sem alias
    monger.NewJoinCollection(profileRepo, "cpf", ""),   // sem alias
)
// result.Data: { "name": "Ana", "cpf": "12345678900", "bio": "...", ... }
```

### JoinAll

Similar ao `Join`, mas retorna **todos** os documentos encontrados em cada coleção (útil para relações 1:N):

```go
// Um usuário pode ter múltiplos pedidos
result, err := monger.JoinAll(ctx, "12345678900",
    monger.NewJoinCollection(usersRepo, "cpf", "user"),
    monger.NewJoinCollection(ordersRepo, "customerCpf", "orders"),
)
if err != nil {
    log.Fatal(err)
}

// result.Data:
// {
//   "user": { "name": "Ana", "cpf": "12345678900" },
//   "orders": [
//     { "orderId": "001", "customerCpf": "12345678900", "total": 100 },
//     { "orderId": "002", "customerCpf": "12345678900", "total": 250 }
//   ]
// }
```

Notas:

- Com `alias` definido: se a coleção retornar 1 documento, vira objeto; se retornar >1, vira array.
- Sem `alias`: o Monger mescla somente o primeiro documento encontrado daquela coleção no resultado.

### JoinWithLookup (Agregação no Servidor)

Usa o operador `$lookup` do MongoDB para fazer o join diretamente no servidor. **Mais eficiente para grandes volumes de dados**.

#### monger.LookupConfig

Cada `monger.LookupConfig` vira um estágio `$lookup` no pipeline. Campos:

- `From`: nome da coleção que será consultada (coleção “externa”).
- `ForeignField`: campo na coleção externa que será comparado com o `localField` da coleção base.
- `As`: nome do campo onde o MongoDB colocará o resultado do `$lookup`.

O Monger remove automaticamente do resultado do `$lookup` o campo usado como chave de relacionamento (`ForeignField`) para evitar repetir dados (ex.: não retorna `customerCpf` se você já tem `cpf` no documento base).

> Observação: no MongoDB, `$lookup` sempre retorna um **array** no campo `As` (mesmo quando a relação é 1:1).

```go
result, err := monger.JoinWithLookup(ctx,
	usersRepo.Collection(),  // coleção base (onde começa a agregação)
	"cpf",                   // localField: campo na coleção base
	"12345678900",           // localValue: valor a buscar na coleção base
	monger.LookupConfig{
		From:         "orders",       // coleção externa
		ForeignField: "customerCpf",  // campo na externa que referencia o CPF
		As:           "orders",       // nome do campo no resultado
	},
	monger.LookupConfig{
		From:         "addresses",
		ForeignField: "ownerCpf",
		As:           "address",
	},
)
if err != nil {
    log.Fatal(err)
}

// result.Data contém o documento base + campos do lookup:
// {
//   "_id": "...",
//   "cpf": "12345678900",
//   "name": "Ana",
//   "orders": [
//     { "orderId": "001", "total": 100 },
//     { "orderId": "002", "total": 250 }
//   ],
//   "address": [
//     { "street": "Rua X" }
//   ]
// }

fmt.Printf("%+v\n", result.Data)
```

Detalhes úteis:

- A agregação começa na `baseCollection` (o primeiro `$match` acontece nela).
- Em `LookupConfig`, `From`, `ForeignField` e `As` são obrigatórios.
- O Monger remove automaticamente o `ForeignField` do resultado do `$lookup` (evita repetir a chave).

### Collection (acessar coleção subjacente)

Para usar `JoinWithLookup`, você pode precisar acessar a coleção MongoDB diretamente:

```go
coll := usersRepo.Collection()
```

### Quando usar cada função?

| Função | Uso recomendado |
|--------|-----------------|
| `Join` | Buscar um documento por coleção (relação 1:1) |
| `JoinAll` | Buscar múltiplos documentos por coleção (relação 1:N) |
| `JoinWithLookup` | Grandes volumes de dados, join feito no servidor MongoDB |

---

## Dicas de uso

- Use `context.Context` com timeout/cancelamento (principalmente em produção).
- Para ordenação, prefira `monger.D` (alias de `bson.D`) pois preserva ordem dos campos.
- Para filtros complexos com operadores que não estão no builder (ex.: `$regex`, `$elemMatch`), você pode misturar com `monger.M` diretamente via `Build()` ou criar um `bson.M` manual.

---

## Licença

Veja [LICENSE](LICENSE).