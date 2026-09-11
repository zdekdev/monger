# FilterBuilder (montagem de filtros)

[← Voltar ao catálogo](../README.md)

Crie filtros com `monger.Filter()` e encadeie comparadores.

## Comparadores

- `Eq(field, val)` / `Equal(field, val)` → `{field: val}`
- `Ne(field, val)` / `NotEqual(field, val)` → `{field: {$ne: val}}`
- `Gt(field, val)` / `GreaterThan(field, val)` → `{field: {$gt: val}}`
- `Gte(field, val)` / `GreaterThanOrEqual(field, val)` → `{field: {$gte: val}}`
- `Lt(field, val)` / `LessThan(field, val)` → `{field: {$lt: val}}`
- `Lte(field, val)` / `LessThanOrEqual(field, val)` → `{field: {$lte: val}}`
- `In(field, vals)` / `InValues(field, vals)` → `{field: {$in: vals}}`

> Observação: `vals` deve ser algo que o driver aceite para `$in` (ex.: `[]string`, `[]int`, etc).

## Operadores lógicos (`And` / `Or`)

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

## Build

`Build()` retorna um `monger.M` (alias de `bson.M`) pronto para uso no driver.

---

Próximo: [ProjectBuilder](project-builder.md)
