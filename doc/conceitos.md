# Conceitos principais

[← Voltar ao catálogo](../README.md)

## Tipos utilitários (`M` e `D`)

O pacote expõe aliases para facilitar a construção de BSON quando necessário:

- `type M = bson.M` (map)
- `type D = bson.D` (slice ordenado)

Isso é útil principalmente para `sort` e para casos onde você quer usar diretamente operadores BSON do driver.

---

## Utilitário `Value`

`monger.Value(v)` retorna um ponteiro para o valor informado. É útil para montar *patch structs* com campos ponteiro em updates parciais, permitindo setar valores zerados (`0`, `""`, `false`).

```go
age := monger.Value(0) // *int apontando para 0
```

---

Próximo: [FilterBuilder](filter-builder.md)
