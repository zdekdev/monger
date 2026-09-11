# Dicas de uso

[← Voltar ao catálogo](../README.md)

- Use `context.Context` com timeout/cancelamento (principalmente em produção).
- Para ordenação, prefira `monger.D` (alias de `bson.D`) pois preserva ordem dos campos.
- Para filtros complexos com operadores que não estão no builder (ex.: `$regex`, `$elemMatch`), você pode misturar com `monger.M` diretamente via `Build()` ou criar um `bson.M` manual.

---

[← Voltar ao catálogo](../README.md)
