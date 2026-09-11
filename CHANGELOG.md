# Changelog

Todas as mudanças relevantes deste projeto são documentadas neste arquivo.

O formato segue [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/)
e o projeto adere ao [Semantic Versioning](https://semver.org/lang/pt-BR/).

## [1.4.0] - 2026-09-11

### Adicionado

- `Repository.UpdateBy`: atualização por campo/valor (ex.: `cpf`, `email`) em vez do `_id`, sem upsert.
- `Repository.Query` com terminais encadeados `Find`, `FindAll`, `FindPaged` e `Join`.
- Join encadeado: `Query.Join`, `JoinRef` e `Ref` (alias derivado do nome da coleção).
- `UpdateByID` passa a aceitar `M`/`D` (raw), inclusive operadores (`$set`, `$unset`, `$inc`, ...).
- Opção `IncludeZeroValues` para updates via struct incluírem campos zerados (`0`, `""`, `false`).
- Reconhecimento da tag `bson:"...,omitempty"` no modo `IncludeZeroValues`.
- Testes unitários de filtros, projeção e update.

### Alterado

- `monger.go` dividido em arquivos por domínio (sem mudança de API pública).
- `go.mod`: `mongo-driver` passa a constar como dependência direta (`go mod tidy`).

### Obsoleto (Deprecated)

- `JoinCollection`, `NewJoinCollection`, `Join` e `JoinAll` — use `Query.Join` com `Ref`.
  Serão removidos na `v2.0.0`.

[1.4.0]: https://github.com/zdekdev/monger/releases/tag/v1.4.0
