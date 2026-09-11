# Repository[T]

[← Voltar ao catálogo](../README.md)

`Repository[T]` encapsula uma `*mongo.Collection` e expõe métodos comuns.

## Criando um repositório

Um `Repository[T]` é um *wrapper* de uma **coleção** do MongoDB:

```go
users := monger.New[User](db, "users") // coleção "users"
```

---

## Query (busca encadeada)

`Repository.Query` cria um builder que reaproveita o mesmo filtro/projeção em vários terminais:

```go
q := users.Query(monger.Filter().Eq("active", true), monger.Select("name", "cpf"))

one, err := q.Find(ctx)                              // 1 documento
many, err := q.FindAll(ctx, 100)                     // N documentos
page, err := q.FindPaged(ctx, 0, 10, nil)            // paginado
joined, err := q.Join(ctx, "cpf", monger.Ref(...))   // união de coleções
```

O terminal `Join` é detalhado em [Join](join.md).

---

## InsertOne

Insere um documento e retorna o `_id` em formato hex string (ObjectID):

```go
id, err := users.InsertOne(ctx, &User{Name: "João"})
```

---

## InsertOneAndUpdate (Upsert)

Realiza um **upsert**: se o documento já existir (baseado no filtro), atualiza apenas os campos diferentes; se não existir, insere o documento completo.

Retorna:
- `id`: o ID do documento (inserido ou existente)
- `isInsert`: `true` se foi uma inserção, `false` se foi uma atualização
- `err`: erro, se houver

> **Quando usar cada função:**
> - Para **inserir** novos documentos: use `InsertOne`.
> - Para **atualizar** por `_id`: use `UpdateByID`.
> - Para **atualizar** por campo único (ex: email, cpf, sku), sem inserir: use `UpdateBy`.
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

---

## Find

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

---

## FindAll

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

**Encadeando múltiplos campos no filtro:**

O `FilterBuilder` suporta encadeamento de múltiplos campos. A busca fuzzy é aplicada apenas em campos `string`; outros tipos (`bool`, `int`, `time.Time`, `ObjectID`) usam igualdade exata.

```go
// Buscar por nome E data de nascimento
clients, err := users.FindAll(ctx, 
    monger.Filter().
        Eq("name", "João").           // fuzzy match no nome
        Eq("birthDate", someDate),    // match exato na data
    nil, 
    100,
)

// Buscar por nome E cidade E status ativo
clients, err = users.FindAll(ctx,
    monger.Filter().
        Eq("name", "Maria").
        Eq("city", "São Paulo").
        Eq("active", true),
    nil,
    50,
)

// Busca complexa com operadores lógicos (OR)
clients, err = users.FindAll(ctx,
    monger.Filter().Or(
        monger.Filter().Eq("name", "João").Eq("city", "Rio"),
        monger.Filter().Eq("name", "Maria").Eq("city", "SP"),
    ),
    nil,
    100,
)
```

**Parâmetros:**
- `ctx`: contexto da operação
- `f`: filtro (opcional, se `nil` retorna todos os documentos)
- `p`: projeção (opcional)
- `limit`: limite de resultados (use `0` para sem limite - **use com cuidado!**)

> **Importante:** Para buscas em grandes coleções, sempre defina um limite razoável para evitar sobrecarga do servidor.
> A busca fuzzy só é aplicada em campos string; campos não-string (como `bool`, `int`, `ObjectID`) usam igualdade exata.

---

## FindPaged (paginação + sort)

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

---

## Count

Conta documentos que satisfazem um filtro:

```go
total, err := users.Count(ctx, monger.Filter().Eq("active", true))
```

---

## Exists

Retorna `true` se existir ao menos um documento que satisfaça o filtro:

```go
ok, err := users.Exists(ctx, monger.Filter().Eq("email", "a@b.com"))
```

---

## UpdateByID (update parcial)

Atualiza parcialmente o documento.

**1) Via struct (padrão):** usa `$set` apenas com campos **não-zerados**.

```go
err := users.UpdateByID(ctx, id, &User{Name: "Novo Nome"})
```

No exemplo acima, somente o campo `Name` será atualizado; os demais campos do documento permanecem como estão.

**2) Valores "zerados" (0, "", false):** em Go não dá para distinguir "campo não informado" de "campo informado com zero" usando apenas um struct comum.
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

**3) Via `M`/`D` (raw):** para controle explícito, inclusive limpando campos (`""`, `0`, `false`).

```go
// Sem operador: o Monger envolve automaticamente em $set.
// Útil para limpar campos sem precisar de um patch struct.
err := users.UpdateByID(ctx, id, monger.M{
	"username": "",
	"host":     "smtp.x",
})

// Com operador: usado como documento de update cru.
// Permite qualquer operador do MongoDB ($set, $unset, $inc, $push, ...).
err = users.UpdateByID(ctx, id, monger.M{
	"$set":   monger.M{"username": ""},
	"$unset": monger.M{"legacy": ""},
})

// Também aceita bson.D (preserva a ordem dos campos).
err = users.UpdateByID(ctx, id, monger.D{
	{Key: "username", Value: ""},
	{Key: "host", Value: "smtp.x"},
})
```

**4) Modo "estado final" com `monger.IncludeZeroValues()`:** para updates via struct, inclui
também os campos zerados (`0`, `""`, `false`). Campos marcados com `bson:"...,omitempty"` e
ponteiros `nil` continuam sendo omitidos.

```go
type User struct {
	Name     string `bson:"name"`
	Age      int    `bson:"age"`
	Active   bool   `bson:"active"`
	Nickname string `bson:"nickname,omitempty"` // omitido se estiver vazio
}

// Name e Age são gravados (Age = 0), Active = false é gravado,
// Nickname vazio é omitido por causa do omitempty.
err := users.UpdateByID(ctx, id, &User{Name: "Ana", Active: false},
	monger.IncludeZeroValues())
```

Sem `IncludeZeroValues()`, o default continua omitindo todos os zerados (update parcial seguro).

> Observação: no caminho via struct ou via mapa **sem operador**, o campo `_id` é ignorado.
> Em documentos com operador (`$set`, `$unset`, ...), o `_id` é de responsabilidade do caller.

---

## UpdateBy (update por campo único)

Atualiza documentos por um **campo/valor** em vez do `_id` — ideal para campos únicos como `cpf`, `email`, `sku`.

Assinatura:

```go
func (r *Repository[T]) UpdateBy(ctx context.Context, field string, value any, update any, opts ...UpdateOption) (int64, error)
```

- `field`: nome do campo usado como chave (obrigatório, não pode ser vazio).
- `value`: valor a casar (ex.: o CPF). Pode ser zero-value (`""`, `0`), pois é um valor de busca.
- `update`: aceita as **mesmas formas do `UpdateByID`** (struct, patch struct com ponteiros, `M`/`D` raw) e as `UpdateOption` (ex.: `monger.IncludeZeroValues()`).
- Retorno: número de **documentos modificados** (`ModifiedCount`). Se o valor gravado for igual ao atual, o Mongo retorna `0`.

Exemplos:

```go
// CPF é único: atualiza o documento com esse cpf
n, err := users.UpdateBy(ctx, "cpf", "12345678900", monger.M{
	"email": "novo@x.com",
})

// Com patch struct (setar valor zerado)
n, err = users.UpdateBy(ctx, "email", "ana@x.com",
	&UserPatch{Name: monger.Value("")})

// Com struct e modo "estado final"
n, err = users.UpdateBy(ctx, "sku", "ABC123",
	&Product{Name: "Notebook", Price: 0},
	monger.IncludeZeroValues())
```

> **UpdateByID × UpdateBy × InsertOneAndUpdate:**
> - `UpdateByID`: atualiza **um** documento pelo `_id`.
> - `UpdateBy`: atualiza por **campo/valor** (ex.: cpf), sem upsert.
> - `InsertOneAndUpdate`: faz **upsert** por filtro (insere se não existir).

---

## DeleteByID

Remove um documento pelo `_id`:

```go
err := users.DeleteByID(ctx, id)
```

---

Próximo: [Join](join.md)
