package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Entity struct {
	ID    primitive.ObjectID `bson:"_id" json:"id"`
	Title string             `bson:"title" json:"title"`
}

type EntityCache struct {
	lock     sync.RWMutex
	Entities []Entity
}

const CACHE_REVALIDATE_INTERVAL = 5 * time.Second

var (
	entityCache       *EntityCache
	enitityCollection *mongo.Collection
)

func main() {
	ctx := context.Background()

	// Init db
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dbClient, err := mongo.Connect(dbCtx, options.Client().ApplyURI(os.Getenv("MONGODB_URI")))
	defer func() {
		if err = dbClient.Disconnect(dbCtx); err != nil {
			log.Fatal(err)
		}
	}()
	enitityCollection = dbClient.Database("main").Collection("entity")

	// Init cache
	entityCache = &EntityCache{}
	entityCache.Entities, _ = getAllEntities()
	go func() {
		timer := time.NewTicker(CACHE_REVALIDATE_INTERVAL)
		defer timer.Stop()

	loop:
		for {
			select {
			case <-timer.C:
				entities, ok := getAllEntities()
				if !ok {
					break
				}

				entityCache.lock.Lock()
				entityCache.Entities = entities
				entityCache.lock.Unlock()
			case <-ctx.Done():
				break loop
			}
		}
	}()

	// Init http server
	router := mux.NewRouter()
	router.HandleFunc("/", GetEntitiesHandler).Methods("GET")
	router.HandleFunc("/", CreateEntityHandler).Methods("POST")
	router.HandleFunc("/{id}", UpdateEntityHandler).Methods("PUT")

	log.Println("Server is starting on port 3000...")
	if err := http.ListenAndServe(":3000", router); err != nil {
		log.Fatal(err)
	}
}

func GetEntitiesHandler(w http.ResponseWriter, r *http.Request) {
	entityCache.lock.RLock()
	entities := entityCache.Entities
	entityCache.lock.RUnlock()

	bytes, _ := json.Marshal(entities)

	w.Header().Add("Content-Type", "application/json")
	w.Write(bytes)
}

func CreateEntityHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "not implemented yet!")
}

func UpdateEntityHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "not implemented yet!")
}

func getAllEntities() ([]Entity, bool) {
	var entities []Entity

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	cursor, err := enitityCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Println("Couldn't fetch entities - ", err.Error())
		return []Entity{}, false
	}
	if err := cursor.All(ctx, &entities); err != nil {
		log.Println("Couldn't decode entities - ", err.Error())
		return []Entity{}, false
	}

	return entities, true
}
