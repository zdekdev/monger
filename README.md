<div align="center">
  <img src="assets/icon.png" alt="Monger" width="96">
  <h1>Monger</h1>
</div>

Monger é um wrapper leve (e genérico) em Go para facilitar operações comuns com MongoDB usando o driver oficial (`go.mongodb.org/mongo-driver`).

Sua filosofia é simples: **faça mais escrevendo menos**. O objetivo é reduzir o boilerplate do dia a dia sem esconder o driver — você continua com controle total quando precisar.

Ele fornece:

- Um `Repository[T]` genérico com operações CRUD, upsert, paginação e contagem.
- Um `FilterBuilder` para montar filtros BSON de forma fluente (sem a verbosidade de `bson.M` direto).
- Um `ProjectBuilder` para projeção de campos (equivalente a `SELECT`/`projection`).
- Funções de `Join` para unir coleções (em memória ou via `$lookup` no servidor).

> O Monger não substitui o driver oficial: ele organiza e reduz boilerplate para casos comuns.

---

## Instalação

```bash
go get github.com/zdekdev/monger
```

```go
import "github.com/zdekdev/monger"
```

Requisitos: Go 1.18+ e MongoDB acessível. Detalhes em [Instalação e Requisitos](doc/instalacao.md).

---

## Documentação

A documentação foi organizada por categoria na pasta [`doc/`](doc/). Use o catálogo abaixo para navegar.

### Começando

| Documento | Descrição |
|---|---|
| [Instalação e Requisitos](doc/instalacao.md) | Como instalar e o que é necessário para usar. |
| [Quickstart](doc/quickstart.md) | Exemplo completo: conexão, repositório e operações básicas. |
| [Conceitos principais](doc/conceitos.md) | Tipos utilitários `M`/`D` e o helper `Value`. |

### Builders

| Documento | Descrição |
|---|---|
| [FilterBuilder](doc/filter-builder.md) | Montagem fluente de filtros: comparadores, `And`/`Or` e `Build`. |
| [ProjectBuilder](doc/project-builder.md) | Projeção de campos com `Select`/`Exclude`. |

### Repositório

| Documento | Descrição |
|---|---|
| [Repository[T]](doc/repository.md) | CRUD completo (`InsertOne`, `InsertOneAndUpdate`, `Find`, `FindAll`, `FindPaged`, `Count`, `Exists`, `UpdateByID`, `UpdateBy`, `DeleteByID`) e o builder `Query`. |

### Junções

| Documento | Descrição |
|---|---|
| [Join](doc/join.md) | Join encadeado com `Query`, além de `Join`, `JoinAll`, `JoinWithLookup`, `JoinCollection`, `JoinRef`/`Ref` e `LookupConfig`. |

### Extras

| Documento | Descrição |
|---|---|
| [Dicas de uso](doc/dicas.md) | Boas práticas de contexto, ordenação e filtros avançados. |

---

## Licença

Veja [LICENSE](LICENSE).
