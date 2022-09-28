package main

import (
	"context"
	"encoding/json"
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

const CACHE_REVALIDATE_INTERVAL = 15 * time.Second

var (
	entityCache      *EntityCache
	entityCollection *mongo.Collection
)

func main() {
	ctx := context.Background()

	// Init db
	dbCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	dbClient, err := mongo.Connect(dbCtx, options.Client().ApplyURI(os.Getenv("MONGODB_URI")))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = dbClient.Disconnect(dbCtx); err != nil {
			log.Fatal(err)
		}
	}()
	entityCollection = dbClient.Database("main").Collection("entity")

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

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entities)
}

func CreateEntityHandler(w http.ResponseWriter, r *http.Request) {
	var payload Entity
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := entityCollection.InsertOne(ctx, &Entity{
		ID:    primitive.NewObjectID(),
		Title: payload.Title,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func UpdateEntityHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	entityId := params["id"]

	objId, err := primitive.ObjectIDFromHex(entityId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var payload Entity
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	update := bson.M{"title": payload.Title}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := entityCollection.FindOneAndUpdate(ctx, bson.M{"_id": objId}, bson.M{"$set": update})

	var parsedResult Entity
	if err := result.Decode(&parsedResult); err != nil {
		if err == mongo.ErrNoDocuments {
			http.Error(w, "can't find specified document", http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var updatedEntity Entity
	if err := entityCollection.FindOne(ctx, bson.M{"_id": objId}).Decode(&updatedEntity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedEntity)
}

func getAllEntities() ([]Entity, bool) {
	var entities []Entity

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cursor, err := entityCollection.Find(ctx, bson.M{})
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
