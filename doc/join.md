# Join (União de Coleções)

[← Voltar ao catálogo](../README.md)

O Monger oferece funções para "juntar" dados de múltiplas coleções usando um **valor em comum** (por exemplo: `cpf`, `email`, `userId`).

Importante: `Join`/`JoinAll` fazem buscas **coleção por coleção** (várias consultas). Para volumes grandes, ou quando você quer que o Mongo faça a união no servidor, prefira `JoinWithLookup`.

## Receita rápida (como pensar)

1) Escolha o `commonValue` (o valor que vai servir de chave). Ex.: `"12345678900"`.

2) Para cada coleção, diga qual **campo** guarda esse valor.

3) Use `alias` (recomendado) para evitar colisão de campos e deixar o resultado organizado.

---

## JoinCollection

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

---

## Join

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

---

## JoinAll

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

---

## JoinWithLookup (Agregação no Servidor)

Usa o operador `$lookup` do MongoDB para fazer o join diretamente no servidor. **Mais eficiente para grandes volumes de dados**.

### monger.LookupConfig

Cada `monger.LookupConfig` vira um estágio `$lookup` no pipeline. Campos:

- `From`: nome da coleção que será consultada (coleção "externa").
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

---

## Collection (acessar coleção subjacente)

Para usar `JoinWithLookup`, você pode precisar acessar a coleção MongoDB diretamente:

```go
coll := usersRepo.Collection()
```

---

## Quando usar cada função?

| Função | Uso recomendado |
|--------|-----------------|
| `Join` | Buscar um documento por coleção (relação 1:1) |
| `JoinAll` | Buscar múltiplos documentos por coleção (relação 1:N) |
| `JoinWithLookup` | Grandes volumes de dados, join feito no servidor MongoDB |

---

Próximo: [Dicas de uso](dicas.md)
