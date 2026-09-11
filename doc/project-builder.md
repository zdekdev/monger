# ProjectBuilder (projeção de campos)

[← Voltar ao catálogo](../README.md)

Use para controlar quais campos voltam do banco:

- `monger.Select("a", "b")` → `{a: 1, b: 1}`
- `monger.Exclude("a", "b")` → `{a: 0, b: 0}`

Exemplo:

```go
u, err := users.Find(ctx, monger.Filter().Eq("_id", oid), monger.Select("name", "age"))
```

---

Próximo: [Repository](repository.md)
