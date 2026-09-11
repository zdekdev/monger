package monger

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func parseBsonTag(tag string) (name string, inline, omitempty bool) {
	if tag == "" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, opt := range parts[1:] {
		switch opt {
		case "inline":
			inline = true
		case "omitempty":
			omitempty = true
		}
	}
	return name, inline, omitempty
}

func buildPartialUpdate(doc any, includeZero bool) (M, error) {
	v := reflect.ValueOf(doc)
	if !v.IsValid() {
		return M{}, nil
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return M{}, nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("modelo precisa ser struct ou ponteiro para struct")
	}

	t := v.Type()
	update := M{}
	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // não-exportado
			continue
		}

		tag, inline, omitempty := parseBsonTag(sf.Tag.Get("bson"))
		if tag == "-" {
			continue
		}

		fv := v.Field(i)
		if inline || (sf.Anonymous && (fv.Kind() == reflect.Struct || (fv.Kind() == reflect.Pointer && fv.Elem().Kind() == reflect.Struct))) {
			sub, err := buildPartialUpdate(fv.Interface(), includeZero)
			if err != nil {
				return nil, err
			}
			for k, val := range sub {
				update[k] = val
			}
			continue
		}

		name := tag
		if name == "" {
			name = sf.Name
		}
		if name == "_id" {
			continue
		}

		// Ponteiros e interfaces:
		//   nil  -> não altera o campo; não-nil -> inclui (mesmo que aponte para valor zerado).
		if fv.Kind() == reflect.Pointer || fv.Kind() == reflect.Interface {
			if fv.IsNil() {
				continue
			}
			if fv.Kind() == reflect.Pointer {
				update[name] = fv.Elem().Interface()
				continue
			}
			update[name] = fv.Interface()
			continue
		}

		// Valores concretos zerados (0, "", false):
		//   default          -> omite;
		//   includeZero      -> inclui, exceto se marcado com bson:"...,omitempty".
		if fv.IsZero() {
			if includeZero && !omitempty {
				update[name] = fv.Interface()
			}
			continue
		}
		update[name] = fv.Interface()
	}
	return update, nil
}

// UpdateOption configura o comportamento de UpdateByID.
type UpdateOption func(*updateConfig)

type updateConfig struct {
	includeZero bool
}

// IncludeZeroValues faz o update incluir também campos com valor zerado (0, "", false)
// ao receber um struct. Campos marcados com bson:"...,omitempty" e ponteiros nil continuam
// sendo omitidos.
//
// Sem esta opção, o default é omitir campos zerados (update parcial seguro).
func IncludeZeroValues() UpdateOption {
	return func(c *updateConfig) { c.includeZero = true }
}

// buildUpdateDocument converte o argumento de update em um documento de update do MongoDB.
//
// Aceita:
//   - struct / *struct: monta $set com os campos via buildPartialUpdate (respeitando opts).
//   - M (bson.M) ou D (bson.D) SEM operador ($): envolvidos em $set (o campo _id é removido).
//   - M (bson.M) ou D (bson.D) COM operador ($set, $unset, $inc, $push, ...): usados como documento cru.
func buildUpdateDocument(update any, opts ...UpdateOption) (any, error) {
	if update == nil {
		return nil, fmt.Errorf("update não pode ser nil")
	}

	switch u := update.(type) {
	case M:
		if len(u) == 0 {
			return nil, fmt.Errorf("nenhum campo para atualizar")
		}
		if isOperatorDocument(u) {
			return u, nil
		}
		set := M{}
		for k, v := range u {
			if k == "_id" {
				continue
			}
			set[k] = v
		}
		if len(set) == 0 {
			return nil, fmt.Errorf("nenhum campo para atualizar")
		}
		return M{"$set": set}, nil
	case D:
		if len(u) == 0 {
			return nil, fmt.Errorf("nenhum campo para atualizar")
		}
		if isOperatorD(u) {
			return u, nil
		}
		set := D{}
		for _, e := range u {
			if e.Key == "_id" {
				continue
			}
			set = append(set, e)
		}
		if len(set) == 0 {
			return nil, fmt.Errorf("nenhum campo para atualizar")
		}
		return D{{Key: "$set", Value: set}}, nil
	}

	cfg := updateConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	doc, err := buildPartialUpdate(update, cfg.includeZero)
	if err != nil {
		return nil, err
	}
	if len(doc) == 0 {
		return nil, fmt.Errorf("nenhum campo para atualizar")
	}
	delete(doc, "_id")
	return M{"$set": doc}, nil
}

// isOperatorD verifica se um bson.D de update contém operadores MongoDB ($set, $unset, ...).
func isOperatorD(d D) bool {
	for _, e := range d {
		if strings.HasPrefix(e.Key, "$") {
			return true
		}
	}
	return false
}

// UpdateByID faz update parcial do documento (UpdateOne).
//
// Formas de uso:
//
//  1. Struct / *struct (padrão): usa $set apenas com campos não-zerados.
//
//     err := users.UpdateByID(ctx, id, &User{Name: "Novo Nome"})
//
//  2. "Patch struct" com ponteiros para setar valores zerados (0, "", false):
//
//     type UserPatch struct {
//     Name   *string `bson:"name"`
//     Active *bool   `bson:"active"`
//     }
//     err := users.UpdateByID(ctx, id, &UserPatch{Name: monger.Value(""), Active: monger.Value(false)})
//
//  3. Mapa (M) ou documento ordenado (D) para controle explícito do valor:
//
//     // Sem operador: envolvido em $set automaticamente.
//     err := users.UpdateByID(ctx, id, M{"username": "", "host": "smtp.x"})
//
//     // Com operador: usado como documento de update cru ($set, $unset, $inc, ...).
//     err := users.UpdateByID(ctx, id, M{"$set": M{"username": ""}, "$unset": M{"old": ""}})
//
//  4. Modo "estado final" (opt-in) com IncludeZeroValues:
//
//     // Inclui também campos zerados do struct, exceto ponteiros nil
//     // e campos marcados com bson:"...,omitempty".
//     err := users.UpdateByID(ctx, id, &User{Name: "Ana", Active: false},
//     monger.IncludeZeroValues())
//
// Nota: no caminho via struct, o campo _id é sempre ignorado.
func (r *Repository[T]) UpdateByID(ctx context.Context, id string, update any, opts ...UpdateOption) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	updateDoc, err := buildUpdateDocument(update, opts...)
	if err != nil {
		return err
	}

	_, err = r.coll.UpdateOne(ctx, M{"_id": oid}, updateDoc)
	return err
}

// UpdateBy atualiza documentos por um campo/valor (ex.: um campo único como cpf, email, sku)
// em vez do _id. Aceita as mesmas formas de update do UpdateByID, incluindo UpdateOption.
//
// O campo e o update são obrigatórios. Retorna o número de documentos modificados.
//
// Exemplo:
//
//	// Atualiza o usuário cujo cpf é único
//	n, err := users.UpdateBy(ctx, "cpf", "12345678900", monger.M{"email": "novo@x.com"})
//
//	// Com patch struct ou IncludeZeroValues
//	n, err = users.UpdateBy(ctx, "email", "ana@x.com", &UserPatch{Name: monger.Value("")})
func (r *Repository[T]) UpdateBy(ctx context.Context, field string, value any, update any, opts ...UpdateOption) (int64, error) {
	if field == "" {
		return 0, fmt.Errorf("field é obrigatório")
	}

	updateDoc, err := buildUpdateDocument(update, opts...)
	if err != nil {
		return 0, err
	}

	res, err := r.coll.UpdateOne(ctx, M{field: value}, updateDoc)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}
