# Quickstart

[← Voltar ao catálogo](../README.md)

Exemplo completo com conexão, criação de repositório e operações básicas.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zdekdev/monger"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Age       int                `bson:"age" json:"age"`
	Active    bool               `bson:"active" json:"active"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}

func main() {
	ctx := context.Background()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = client.Disconnect(ctx) }()

	db := client.Database("app")
	users := monger.New[User](db, "users")

	// Insert
	u := User{Name: "Ana", Age: 29, Active: true, CreatedAt: time.Now()}
	id, err := users.InsertOne(ctx, &u)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("inserted id:", id)

	// Converter string ID para ObjectID
	oid, _ := primitive.ObjectIDFromHex(id)

	// Find por ID (com projeção opcional)
	found, err := users.Find(ctx, monger.Filter().Eq("_id", oid), monger.Select("name", "age"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("found: %+v\n", found)

	// FindAll com filtro (busca fuzzy)
	list, err := users.FindAll(ctx,
		monger.Filter().
			Eq("active", true).
			Gte("age", 18),
		nil,
		100, // limite de resultados
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("total active adults:", len(list))
}
```

---

Próximo: [Conceitos principais](conceitos.md)
