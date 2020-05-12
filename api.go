package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/justinas/alice"
	"github.com/memcachier/mc"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	sceneIdLength         = 6
	clientIdLength        = 6
	maxRegisterIterations = 10
	maxConnectIterations  = 10
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var Memcached *mc.Client
var WebSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Client struct {
	Id         string
	SceneId    string
	Connection *websocket.Conn
	WriteQueue chan []byte
}

const (
	writeTimeout   = 1 * time.Second
	pingPeriod     = 1 * time.Second
	pongTimeout    = 5 * time.Second
	maxMessageSize = 32
)

func NewClient(id string, sceneId string, connection *websocket.Conn) *Client {
	client := &Client{
		Id:         id,
		SceneId:    sceneId,
		Connection: connection,
		WriteQueue: make(chan []byte),
	}
	if err := client.Connection.SetReadDeadline(time.Now().Add(pongTimeout)); err != nil {
		log.Printf("[Client %s] Error during setting read deadline: %s", client.Id, err)
		return nil
	}

	client.Connection.SetReadLimit(maxMessageSize)
	if err := client.Connection.SetReadDeadline(time.Now().Add(pongTimeout)); err != nil {
		log.Printf("[Client %s] Error during setting read deadline: %s", client.Id, err)
		if err := client.Connection.Close(); err != nil {
			log.Printf("[Client %s] Error during connection closing: %s", client.Id, err)
		}
	}
	client.Connection.SetPongHandler(func(string) error {
		if err := client.Connection.SetReadDeadline(time.Now().Add(pongTimeout)); err != nil {
			log.Printf("[Client %s] Error during setting read deadline: %s", client.Id, err)
			return err
		}
		return nil
	})

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop()
			<-registry.DeleteClient(client)
			if err := client.Connection.Close(); err != nil {
				log.Printf("[Client %s] Error during connection closing: %s", client.Id, err)
			}
		}()
		for {
			select {
			case message, ok := <-client.WriteQueue:
				_ = client.Connection.SetWriteDeadline(time.Now().Add(writeTimeout))
				if !ok {
					if err := client.Connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
						log.Printf("[Client %s] Error during sending CloseMessage: %s", client.Id, err)
					}
					return
				}
				writer, err := client.Connection.NextWriter(websocket.TextMessage)
				if err != nil {
					log.Printf("[Client %s] Writer creation error: %s", client.Id, err)
					return
				}
				_, err = writer.Write(message)
				if err != nil {
					log.Printf("[Client %s] Write error: %s", client.Id, err)
					return
				}
				if err := writer.Close(); err != nil {
					log.Printf("[Client %s] Writer close error: %s", client.Id, err)
					return
				}
			case <-ticker.C:
				_ = client.Connection.SetWriteDeadline(time.Now().Add(writeTimeout))
				if err := client.Connection.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("[Client %s] Ping write error: %s", client.Id, err)
					return
				}
			}
		}
	}()

	go func() {
		defer func() {
			<-registry.DeleteClient(client)
			if err := client.Connection.Close(); err != nil {
				log.Printf("[Client %s] Error during connection closing: %s", client.Id, err)
			}
		}()
		for {
			_, _, err := client.Connection.ReadMessage()
			if err != nil {
				log.Printf("[Client %s] Read error: %s", client.Id, err)
				return
			}
		}
	}()

	return client
}

func (client *Client) SendJSON(object ObjectContainer) error {
	marshalled, err := json.Marshal(object)
	if err != nil {
		return err
	}
	client.WriteQueue <- marshalled
	return nil
}

type ModificationType uint

const (
	Add            ModificationType = iota
	Delete         ModificationType = iota
	SceneBroadcast ModificationType = iota
)

type SceneBroadcastInfo struct {
	SceneId string
	Object  *ObjectContainer
}

type SceneBroadcastError struct {
	Err      error
	ClientId string
}

func (err SceneBroadcastError) Error() string {
	return fmt.Sprintf("error '%s' on client '%s'", err.Err, err.ClientId)
}

type ClientModification struct {
	Type            ModificationType
	Client          *Client
	AuxData         interface{}
	ResponseChannel chan interface{}
}

type Registry struct {
	ConnectedClients          map[string]*Client
	Scenes                    map[string]map[string]*Client
	ClientModificationChannel chan ClientModification
}

var registry *Registry

func ConfigureRegistry() {
	registry = NewRegistry()
}

func NewRegistry() *Registry {
	registry := Registry{}
	registry.ConnectedClients = make(map[string]*Client)
	registry.Scenes = make(map[string]map[string]*Client)
	registry.ClientModificationChannel = make(chan ClientModification)
	registry.StartModificationServant()
	return &registry
}

func (registry *Registry) StartModificationServant() {
	go func(registry *Registry) {
		for mod := range registry.ClientModificationChannel {
			switch mod.Type {
			case Add:
				registry.ConnectedClients[mod.Client.Id] = mod.Client
				if _, present := registry.Scenes[mod.Client.SceneId]; !present {
					registry.Scenes[mod.Client.SceneId] = make(map[string]*Client)
				}
				registry.Scenes[mod.Client.SceneId][mod.Client.Id] = mod.Client
				mod.ResponseChannel <- 1
			case Delete:
				delete(registry.ConnectedClients, mod.Client.Id)
				if len(registry.Scenes[mod.Client.SceneId]) == 1 {
					delete(registry.Scenes, mod.Client.SceneId)
				} else {
					delete(registry.Scenes[mod.Client.SceneId], mod.Client.Id)
				}
				mod.ResponseChannel <- 1
			case SceneBroadcast:
				info := mod.AuxData.(*SceneBroadcastInfo)
				if _, present := registry.Scenes[info.SceneId]; !present {
					err := SceneBroadcastError{
						Err:      errors.New(fmt.Sprintf("Scene %s not found", info.SceneId)),
						ClientId: "",
					}
					mod.ResponseChannel <- []SceneBroadcastError{err}
					break
				}
				var broadcastErrors []SceneBroadcastError
				for _, client := range registry.Scenes[info.SceneId] {
					err := client.Connection.WriteJSON(*info.Object) // todo: pipe this data to write queue to eliminate data race
					if err != nil {
						broadcastErrors = append(broadcastErrors, SceneBroadcastError{
							Err:      err,
							ClientId: client.Id,
						})
					}
				}
				mod.ResponseChannel <- broadcastErrors
			}
		}
	}(registry)
}

func (registry *Registry) ModifyClient(modificationType ModificationType, client *Client, auxData interface{}) chan interface{} {
	modification := ClientModification{
		Type:            modificationType,
		Client:          client,
		AuxData:         auxData,
		ResponseChannel: make(chan interface{}),
	}
	registry.ClientModificationChannel <- modification
	return modification.ResponseChannel
}

func (registry *Registry) AddClient(client *Client) chan interface{} {
	return registry.ModifyClient(Add, client, nil)
}

func (registry *Registry) DeleteClient(client *Client) chan interface{} {
	return registry.ModifyClient(Delete, client, nil)
}

func (registry *Registry) SendSceneBroadcast(info *SceneBroadcastInfo) chan interface{} {
	return registry.ModifyClient(SceneBroadcast, nil, info)
}

func generateRandomId(length int) string {
	mc.DefaultConfig()
	res := make([]rune, length)
	for i := range res {
		res[i] = letters[rand.Intn(len(letters))]
	}
	return string(res)
}

func recoverHandlerFunc(next http.Handler) http.Handler {
	fn := func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %+v", err)
				http.Error(writer, http.StatusText(500), 500)
			}
		}()
		next.ServeHTTP(writer, request)
	}
	return http.HandlerFunc(fn)
}

func registerHandler(writer http.ResponseWriter, _ *http.Request) {
	var id string
	var i int
	for i = 0; i < maxRegisterIterations; i++ {
		id = generateRandomId(sceneIdLength)
		val, _, _, err := Memcached.Get(id)
		if err != nil && err.(*mc.Error).Status == mc.StatusNotFound {
			break
		}
		log.Printf("[register] Value: %v, Error: %v", val, err)
	}
	if i == maxRegisterIterations {
		log.Printf("[register] Exceeded max iteration")
		http.Error(writer, http.StatusText(503), 503)
		return
	}
	_, err := Memcached.Set(id, "", 0, 0, 0)
	if err != nil {
		log.Printf("[register] Memcached set error: %s\n", err)
		http.Error(writer, http.StatusText(500), 500)
		return
	}
	_, err = writer.Write([]byte(id))
	if err != nil {
		log.Printf("[register] Response writer error: %s\n", err)
		http.Error(writer, http.StatusText(500), 500)
		return
	}
}

func sendHandler(writer http.ResponseWriter, request *http.Request) {
	sceneId := mux.Vars(request)["id"]
	requestBodyBytes, err := ioutil.ReadAll(request.Body)

	decoder := json.NewDecoder(bytes.NewReader(requestBodyBytes))
	var requestData ObjectContainer
	err = decoder.Decode(&requestData)
	if err != nil {
		log.Printf("[send] Failed to decode json: %s", err)
		http.Error(writer, http.StatusText(400), 400)
		return
	}

	objectId := strconv.FormatInt(requestData.Id, 10)
	storeKey := strings.Join([]string{sceneId, objectId}, "/")

	// update cache
	switch requestData.Method {
	case "set":
		val, _, _, err := Memcached.Get(sceneId)
		if err != nil {
			log.Printf("[send] Failed to retrieve key for id %s: %s", sceneId, err)
			http.Error(writer, http.StatusText(422), 422)
			return
		}
		identifiers, err := DeserializeIdentifiers(val)
		if err != nil {
			log.Printf("[send] Error during identifiers deserialization from '%s': %s", val, err)
			http.Error(writer, http.StatusText(500), 500)
			return
		}
		identifiers[requestData.Id] = struct{}{}
		serialized, err := SerializeObjectIdentifiers(identifiers)
		if err != nil {
			fmt.Printf("[send] Error during identifiers serialization: %s", err)
			http.Error(writer, http.StatusText(500), 500)
			return
		}
		_, err = Memcached.Set(sceneId, serialized, 0, 0, 0)
		if err != nil {
			log.Printf("[send] Failed to store key %s with value %s in cache: %s", storeKey, val, err)
			http.Error(writer, http.StatusText(500), 500)
			return
		}
		_, err = Memcached.Set(storeKey, string(requestBodyBytes), 0, 0, 0)
		if err != nil {
			log.Printf("[send] Failed to store request body for key %s, body %s: %s", storeKey, string(requestBodyBytes), err)
			http.Error(writer, http.StatusText(500), 500)
			return
		}
	//case "delete":
	//	val, _, _, err := Memcached.Get(sceneId)
	//	if err != nil {
	//		log.Printf("[send] Failed to retrieve key for id %s: %s", sceneId, err)
	//		http.Error(writer, http.StatusText(422), 422)
	//		return
	//	}
	//	identifiers, err := DeserializeIdentifiers(val)
	//	if err != nil {
	//
	//	}
	default:
		log.Printf("[send] Not implemented method requested: %s", requestData.Method)
	}

	broadcastInfo := SceneBroadcastInfo{
		SceneId: sceneId,
		Object:  &requestData,
	}
	errs := (<-registry.SendSceneBroadcast(&broadcastInfo)).([]SceneBroadcastError)
	if len(errs) > 0 {
		log.Printf("[send] Got %d errors during object transfer", len(errs))
		for _, err = range errs {
			log.Printf("[send] Object transfer error: %s", err)
		}
	}
}

func listenHandler(writer http.ResponseWriter, request *http.Request) {
	var clientId string
	for i := 0; i < maxConnectIterations; i++ {
		clientId = generateRandomId(clientIdLength)
		if _, present := registry.ConnectedClients[clientId]; present {
			log.Printf("[listen] Client with id %s already exists", clientId)
			continue
		}
		log.Printf("[listen] New client id %s", clientId)
		break
	}
	if _, present := registry.ConnectedClients[clientId]; present {
		log.Printf("[listen] Unable to generate new id for client")
		http.Error(writer, http.StatusText(503), 503)
		return
	}

	connection, err := WebSocketUpgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Printf("[listen] Connection upgrade failed: %s", err)
		return
	}
	sceneId := mux.Vars(request)["id"]
	client := NewClient(clientId, sceneId, connection)
	<-registry.AddClient(client)

	// send cached objects
	val, _, _, err := Memcached.Get(sceneId)
	if err != nil {
		log.Printf("[send] Failed to retrieve key for id %s: %s", sceneId, err)
		http.Error(writer, http.StatusText(422), 422)
		return
	}
	identifiers, err := DeserializeIdentifiers(val)
	if err != nil {
		log.Printf("[send] Error during identifiers deserialization from '%s': %s", val, err)
		http.Error(writer, http.StatusText(500), 500)
		return
	}
	for objectId := range identifiers {
		storeKey := strings.Join([]string{sceneId, strconv.FormatInt(objectId, 10)}, "/")
		val, _, _, err := Memcached.Get(storeKey)
		if err != nil {
			log.Printf("[send] Failed to retrieve value for id %s: %s", storeKey, err)
			continue
		}
		client.WriteQueue <- []byte(val)
	}
	if len(identifiers) == 0 {
		object := ObjectContainer{
			Id:      0,
			Type:    "clear",
			Method:  "",
			Content: nil,
		}
		err := client.SendJSON(object)
		if err != nil {
			log.Printf("[send] Failed to send clear json: %s", err)
		}
	}
}

func ConfigureCache() {
	username := os.Getenv("MEMCACHIER_USERNAME")
	password := os.Getenv("MEMCACHIER_PASSWORD")
	servers := os.Getenv("MEMCACHIER_SERVERS")

	config := mc.DefaultConfig()
	config.Hasher = mc.NewModuloHasher()
	config.Retries = 2
	config.RetryDelay = 200 * time.Millisecond
	config.Failover = true
	config.ConnectionTimeout = 2 * time.Second
	config.DownRetryDelay = 60 * time.Second
	config.PoolSize = 1
	config.TcpKeepAlive = true
	config.TcpKeepAlivePeriod = 60 * time.Second
	config.TcpNoDelay = true

	Memcached = mc.NewMCwithConfig(servers, username, password, config)
}

func SetupHandles() *mux.Router {
	router := mux.NewRouter()
	commonHandlers := alice.New(recoverHandlerFunc)
	router.Handle("/api/v1/register", commonHandlers.ThenFunc(registerHandler)).Methods("GET")
	router.Handle(
		fmt.Sprintf("/api/v1/{id:[a-zA-Z]{%d}}/send", sceneIdLength),
		commonHandlers.ThenFunc(sendHandler),
	).Methods("POST")
	router.Handle(
		fmt.Sprintf("/api/v1/{id:[a-zA-Z]{%d}}/listen", sceneIdLength),
		commonHandlers.ThenFunc(listenHandler),
	)
	router.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./static/"))))
	return router
}
