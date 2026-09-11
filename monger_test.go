package monger

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type testUser struct {
	ID       string `bson:"_id"`
	Name     string `bson:"name"`
	Age      int    `bson:"age"`
	Active   bool   `bson:"active"`
	Nickname string `bson:"nickname,omitempty"`
	Ptr      *int   `bson:"ptr"`
}

func TestValue(t *testing.T) {
	v := Value(0)
	if v == nil || *v != 0 {
		t.Fatalf("Value(0) = %v", v)
	}
}

func TestFilterBuilderComparators(t *testing.T) {
	got := Filter().
		Eq("a", 1).
		Gte("b", 2).
		Ne("c", "x").
		In("d", []int{1, 2}).
		Build()

	want := M{
		"a": 1,
		"b": M{"$gte": 2},
		"c": M{"$ne": "x"},
		"d": M{"$in": []int{1, 2}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestFilterBuilderLogical(t *testing.T) {
	got := Filter().And(
		Filter().Eq("a", 1),
		Filter().Or(
			Filter().Eq("b", 2),
			Filter().Eq("c", 3),
		),
	).Build()

	want := M{
		"$and": []M{
			{"a": 1},
			{"$or": []M{{"b": 2}, {"c": 3}}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestProjectBuilder(t *testing.T) {
	if got, want := Select("a", "b").Build(), (M{"a": 1, "b": 1}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Select got=%#v want=%#v", got, want)
	}
	if got, want := Exclude("a").Build(), (M{"a": 0}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Exclude got=%#v want=%#v", got, want)
	}
}

func TestParseBsonTag(t *testing.T) {
	tests := []struct {
		tag               string
		name              string
		inline, omitempty bool
	}{
		{"", "", false, false},
		{"name", "name", false, false},
		{"name,omitempty", "name", false, true},
		{",inline", "", true, false},
		{"_,inline,omitempty", "_", true, true},
	}
	for _, tt := range tests {
		name, inline, omitempty := parseBsonTag(tt.tag)
		if name != tt.name || inline != tt.inline || omitempty != tt.omitempty {
			t.Fatalf("parseBsonTag(%q) = (%q,%v,%v), want (%q,%v,%v)",
				tt.tag, name, inline, omitempty, tt.name, tt.inline, tt.omitempty)
		}
	}
}

func TestBuildPartialUpdate(t *testing.T) {
	zero := 0

	tests := []struct {
		name        string
		doc         any
		includeZero bool
		want        M
	}{
		{"default omite zerados", &testUser{Name: "Ana"}, false, M{"name": "Ana"}},
		{"includeZero inclui zerados", &testUser{Name: "Ana"}, true,
			M{"name": "Ana", "age": 0, "active": false}},
		{"omitempty ignorado no default", &testUser{Nickname: ""}, false, M{}},
		{"ponteiro nao-nil inclui zero", &testUser{Ptr: &zero}, false, M{"ptr": 0}},
		{"ponteiro nil ignorado", &testUser{}, true, M{"name": "", "age": 0, "active": false}},
		{"id ignorado", &testUser{ID: "x", Name: "Ana"}, true, M{"name": "Ana", "age": 0, "active": false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildPartialUpdate(tt.doc, tt.includeZero)
			if err != nil {
				t.Fatalf("erro: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func TestBuildUpdateDocument(t *testing.T) {
	tests := []struct {
		name   string
		update any
		opts   []UpdateOption
		want   any
	}{
		{
			"struct default",
			&testUser{Name: "Ana"},
			nil,
			M{"$set": M{"name": "Ana"}},
		},
		{
			"struct includeZero",
			&testUser{Name: "Ana", Active: false},
			[]UpdateOption{IncludeZeroValues()},
			M{"$set": M{"name": "Ana", "age": 0, "active": false}},
		},
		{
			"mapa sem operador vira set",
			M{"_id": "x", "username": ""},
			nil,
			M{"$set": M{"username": ""}},
		},
		{
			"mapa com operador cru",
			M{"$set": M{"username": ""}, "$unset": M{"legacy": ""}},
			nil,
			M{"$set": M{"username": ""}, "$unset": M{"legacy": ""}},
		},
		{
			"bson.D sem operador vira set",
			D{{Key: "username", Value: ""}},
			nil,
			D{{Key: "$set", Value: D{{Key: "username", Value: ""}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildUpdateDocument(tt.update, tt.opts...)
			if err != nil {
				t.Fatalf("erro: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func TestBuildUpdateDocumentErros(t *testing.T) {
	cases := []any{
		nil,
		M{},
		M{"_id": "x"},
		&testUser{},
	}
	for _, c := range cases {
		if _, err := buildUpdateDocument(c); err == nil {
			t.Fatalf("esperava erro para %#v", c)
		}
	}
}

func TestEscapeRegex(t *testing.T) {
	if got, want := escapeRegex("a.b*c"), `a\.b\*c`; got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestConvertToFuzzyFilter(t *testing.T) {
	got := convertToFuzzyFilter(M{
		"name": "Ana",
		"age":  30,
		"tags": M{"$in": []string{"a", "b"}},
	})
	want := M{
		"name": primitive.Regex{Pattern: "Ana", Options: "i"},
		"age":  30,
		"tags": M{"$in": []string{"a", "b"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestJoinProjection(t *testing.T) {
	tests := []struct {
		name  string
		p     M
		field string
		want  M
	}{
		{"inclusiva adiciona localField", M{"name": 1}, "cpf", M{"name": 1, "cpf": 1}},
		{"exclusiva mantem", M{"secret": 0}, "cpf", M{"secret": 0}},
		{"exclusiva removendo localField", M{"cpf": 0}, "cpf", M{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinProjection(tt.p, tt.field); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got=%#v want=%#v", got, tt.want)
			}
		})
	}
}

func TestIsProjectionOn(t *testing.T) {
	for _, v := range []any{1, int32(1), int64(1), true} {
		if !isProjectionOn(v) {
			t.Fatalf("isProjectionOn(%#v) deveria ser true", v)
		}
	}
	for _, v := range []any{0, int32(0), int64(0), false, "x"} {
		if isProjectionOn(v) {
			t.Fatalf("isProjectionOn(%#v) deveria ser false", v)
		}
	}
}
