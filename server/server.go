package main

import (
	context "context"
	"errors"
	"log"
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/proto"
)

const (
	sceneIdLength             = 6
	clientIdLength            = 6
	authenticatorLength       = 8
	drawingIdLength           = 22
	maxRegisterIterations     = 1000
	maxDrawingsInPollResponse = 32

	scenesCollectionName   = "scenes"
	drawingsCollectionName = "drawings"

	sceneIdKey       = "scene_id"
	authenticatorKey = "authenticator"
	timestampKey     = "timestamp"
	drawingKey       = "drawing"
	drawingIdKey     = "drawing_id"
)

var (
	runes       = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	mongoClient *mongo.Client
	db          *mongo.Database

	internalError         = errors.New("Internal error")
	maxIterationsError    = errors.New("Exceeded max number of iterations")
	authentificationError = errors.New("Authentification error")
)

func generateRandomId(length int) string {
	res := make([]rune, length)
	for i := range res {
		res[i] = runes[rand.Intn(len(runes))]
	}
	return string(res)
}

func defaultContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func checkAuthenticator(scenes *mongo.Collection, sceneId string, authenticator string) error {
	ctx, cancel := defaultContext()
	defer cancel()
	result := scenes.FindOne(ctx, bson.M{sceneIdKey: sceneId})
	if err := result.Err(); err != nil {
		log.Printf("checkAuthenticator: FindOne error %s\n", err)
		return internalError
	}
	var scene bson.M
	if err := result.Decode(&scene); err != nil {
		log.Printf("checkAuthenticator: Decode error %s\n", err)
		return internalError
	}
	if scene[authenticatorKey] != authenticator {
		log.Printf("checkAuthenticator: Incorrect authenticator %s with SceneId %s", authenticator, sceneId)
		return authentificationError
	}
	return nil
}

type drawerServer struct {
	UnimplementedDrawerServer
}

func (s drawerServer) Register(ctx context.Context, req *RegisterRequest) (*RegisterResult, error) {
	scenes := db.Collection(scenesCollectionName)

	sceneObject := make(bson.M)
	var i int
	for i = 0; i < maxRegisterIterations; i++ {
		sceneObject[sceneIdKey] = generateRandomId(sceneIdLength)

		ctx, cancel := defaultContext()
		defer cancel()

		result := scenes.FindOne(ctx, sceneObject)
		err := result.Err()
		if err == mongo.ErrNoDocuments {
			break
		} else if err != nil {
			log.Printf("Register: FindOne error %s\n", err)
			return nil, internalError
		}
		log.Printf("Generated scene_id %s already registered in db", sceneObject[sceneIdKey].(string))
	}
	if i == maxRegisterIterations {
		log.Printf("Register: Exceeded max number of iterations\n")
		return nil, maxIterationsError
	}

	sceneObject[authenticatorKey] = generateRandomId(authenticatorLength)

	ctx, cancel := defaultContext()
	defer cancel()

	result, err := scenes.InsertOne(ctx, sceneObject)
	if err != nil {
		log.Printf("Register: InsertOne error %s\n", err)
		return nil, internalError
	}
	log.Printf("Register: Created scene with ObjectID %s and SceneId %s\n", result.InsertedID, sceneObject[sceneIdKey].(string))

	return &RegisterResult{
		SceneId:       sceneObject[sceneIdKey].(string),
		Authenticator: sceneObject[authenticatorKey].(string),
	}, nil
}

func (s drawerServer) Delist(ctx context.Context, req *DelistRequest) (*DelistResult, error) {
	scenes := db.Collection(scenesCollectionName)

	if err := checkAuthenticator(scenes, req.SceneId, req.Authenticator); err != nil {
		return nil, err
	}

	ctx, cancel := defaultContext()
	defer cancel()

	result, err := scenes.DeleteOne(ctx, bson.M{sceneIdKey: req.SceneId})
	if err != nil {
		log.Printf("Delist: DeleteOne error %s\n", err)
		return nil, internalError
	}
	log.Printf("Delist: Deleted %d scene(s) by id %s\n", result.DeletedCount, req.SceneId)

	return &DelistResult{}, nil
}

func (s drawerServer) Draw(ctx context.Context, req *DrawRequest) (*DrawResult, error) {
	scenes := db.Collection(scenesCollectionName)

	if err := checkAuthenticator(scenes, req.SceneId, req.Authenticator); err != nil {
		return nil, err
	}

	drawings := db.Collection(drawingsCollectionName)
	ctx, cancel := defaultContext()
	defer cancel()

	bytes, err := proto.Marshal(req.Drawable)
	if err != nil {
		log.Printf("Draw: Marshal error %s\n", err)
		return nil, internalError
	}

	drawingId := generateRandomId(drawingIdLength) // this should be unique (at least as unique as uuid)
	result, err := drawings.InsertOne(ctx, bson.M{
		sceneIdKey:   req.SceneId,
		timestampKey: time.Now(),
		drawingKey:   bytes,
		drawingIdKey: drawingId,
	})
	if err != nil {
		log.Printf("Draw: InsertOne error %s\n", err)
		return nil, internalError
	}
	log.Printf("Draw: Inserted new drawing with SceneId %s with ObjectID %s", req.SceneId, result.InsertedID)

	return &DrawResult{
		DrawingId: drawingId,
	}, nil
}

func (s drawerServer) Remove(ctx context.Context, req *RemoveRequest) (*RemoveResult, error) {
	scenes := db.Collection(scenesCollectionName)

	if err := checkAuthenticator(scenes, req.SceneId, req.Authenticator); err != nil {
		return nil, err
	}

	drawings := db.Collection(drawingsCollectionName)
	ctx, cancel := defaultContext()
	defer cancel()

	result, err := drawings.DeleteOne(ctx, bson.M{
		drawingIdKey: req.DrawingId,
	})
	if err != nil {
		log.Printf("Remove: DeleteOne error %s\n", err)
		return nil, internalError
	}
	log.Printf("Remove: Deleted %d drawing(s) by DrawingId %s", result.DeletedCount, req.DrawingId)

	return &RemoveResult{}, nil
}

func (s drawerServer) Clear(ctx context.Context, req *ClearRequest) (*ClearResult, error) {
	scenes := db.Collection(scenesCollectionName)

	if err := checkAuthenticator(scenes, req.SceneId, req.Authenticator); err != nil {
		return nil, err
	}

	drawings := db.Collection(drawingsCollectionName)
	ctx, cancel := defaultContext()
	defer cancel()

	result, err := drawings.DeleteMany(ctx, bson.M{
		sceneIdKey: req.SceneId,
	})
	if err != nil {
		log.Printf("Clear: DeleteMany error %s\n", err)
		return nil, internalError
	}
	log.Printf("Clear: Deleted %d drawing(s) by SceneId %s", result.DeletedCount, req.SceneId)

	return &ClearResult{}, nil
}

func (s drawerServer) Poll(ctx context.Context, req *PollRequest) (*PollResult, error) {
	// scenes := db.Collection(scenesCollectionName)

	// if err := checkAuthenticator(scenes, req.SceneId, req.Authenticator); err != nil {
	// 	return nil, err
	// }

	drawings := db.Collection(drawingsCollectionName)
	ctx, cancel := defaultContext()
	defer cancel()

	cursor, err := drawings.Find(ctx, bson.M{
		sceneIdKey:   req.SceneId,
		timestampKey: bson.M{"$gt": time.Unix(int64(req.AfterTimestamp), 999999999)},
	}, options.Find().SetSort(bson.M{timestampKey: 1}))
	if err != nil {
		log.Printf("Poll: Find error %s\n", err)
		return nil, internalError
	}
	defer cursor.Close(ctx)

	var result PollResult
	var i int = 0
	var lastTimestamp int64 = 0
	for cursor.Next(ctx) {
		i += 1
		if i > maxDrawingsInPollResponse {
			break
		}
		var document bson.M
		if err = cursor.Decode(&document); err != nil {
			log.Printf("Poll: Decode error %s\n", err)
			return nil, internalError
		}
		lastTimestamp = document[timestampKey].(primitive.DateTime).Time().Unix()

		var drawable Drawable
		if err = proto.Unmarshal(document[drawingKey].(primitive.Binary).Data, &drawable); err != nil {
			log.Printf("Poll: Unmarshal error %s\n", err)
			return nil, internalError
		}

		result.Drawings = append(result.Drawings, &drawable)
	}
	result.LastTimestamp = uint64(lastTimestamp)

	return &result, nil
}

func newDrawerServer() *drawerServer {
	return &drawerServer{}
}

func initMongo() {
	mongoClient, err := mongo.NewClient(options.Client().ApplyURI("mongodb://localhost:27017"))
	EnsureNoError(err)

	ctx, cancel := defaultContext()
	defer cancel()
	EnsureNoError(mongoClient.Connect(ctx))

	db = mongoClient.Database("Visualization")
}

func stopMongo() {
	ctx, cancel := defaultContext()
	defer cancel()
	mongoClient.Disconnect(ctx)
}
