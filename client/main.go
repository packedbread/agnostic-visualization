package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	grpc "google.golang.org/grpc"
)

var address = flag.String("address", "localhost:10101", "The server address in the format of host:port")
var action = flag.String("action", "", "RPC handle name")
var sceneId = flag.String("scene-id", "", "SceneId after registration")
var authenticator = flag.String("authenticator", "", "Authenticator after registration")
var drawingType = flag.String("drawing-type", "", "Drawable name")
var drawingData = flag.String("drawing-data", "", "Serialized drawing data -- string of numbers")
var drawingId = flag.String("drawing-id", "", "Drawing Id")

var client DrawerClient

func EnsureNoError(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func EnsureNotEmpty(str *string, message string) {
	if *str == "" {
		log.Fatalln(message)
	}
}

func NewClient() (DrawerClient, *grpc.ClientConn) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())

	conn, err := grpc.Dial(*address, opts...)
	EnsureNoError(err)

	fmt.Println("Successfully dialed")

	return NewDrawerClient(conn), conn
}

func RegisterHandler() {
	registerResult, err := client.Register(context.Background(), &RegisterRequest{})
	EnsureNoError(err)

	fmt.Printf("Registered scene with SceneId: %s, Authenticator: %s\n", registerResult.SceneId, registerResult.Authenticator)
}

func DrawHandler() {
	EnsureNotEmpty(sceneId, "Empty scene-id")
	EnsureNotEmpty(authenticator, "Empty authenticator")
	EnsureNotEmpty(drawingType, "Empty drawing-type")
	EnsureNotEmpty(drawingData, "Empty drawing-data")

	var drawable *Drawable

	stringData := strings.Split(*drawingData, " ")
	var floatData []float64
	for _, str := range stringData {
		num, err := strconv.ParseFloat(str, 64)
		EnsureNoError(err)
		floatData = append(floatData, num)
	}

	if *drawingType == "line" {
		drawable = &Drawable{
			Content: &Drawable_Line{
				Line: &Line{
					From: &Point{X: floatData[0], Y: floatData[1]},
					To:   &Point{X: floatData[2], Y: floatData[3]},
				},
			},
		}
	} else if *drawingType == "rectangle" {
		drawable = &Drawable{
			Content: &Drawable_Rectangle{
				Rectangle: &Rectangle{
					LowerLeft:  &Point{X: floatData[0], Y: floatData[1]},
					UpperRight: &Point{X: floatData[2], Y: floatData[3]},
				},
			},
		}
	} else if *drawingType == "circle" {
		drawable = &Drawable{
			Content: &Drawable_Circle{
				Circle: &Circle{
					Center: &Point{X: floatData[0], Y: floatData[1]},
					Radius: floatData[2],
				},
			},
		}
	} else {
		EnsureNoError(errors.New(fmt.Sprintf("Unknown drawing type %s", *drawingType)))
	}

	drawResult, err := client.Draw(context.Background(), &DrawRequest{
		SceneId:       *sceneId,
		Authenticator: *authenticator,
		Drawable:      drawable,
	})
	EnsureNoError(err)
	fmt.Printf("Drew %s on scene with SceneId %s, DrawingId %s\n", *drawingType, *sceneId, drawResult.DrawingId)
}

func PollHandler() {
	// result, err := client.Poll(context.Background(), &PollRequest{
	// 	SceneId:        registerResult.SceneId,
	// 	Authenticator:  registerResult.Authenticator,
	// 	AfterTimestamp: 0,
	// })
	// EnsureNoError(err)
	// fmt.Printf("Polled drawings from scene with SceneId %s, got %d drawings\n", registerResult.SceneId, len(result.Drawings))
}

func RemoveHandler() {
	EnsureNotEmpty(sceneId, "Empty scene-id")
	EnsureNotEmpty(authenticator, "Empty authenticator")
	EnsureNotEmpty(drawingId, "Empty drawing-id")

	_, err := client.Remove(context.Background(), &RemoveRequest{
		SceneId:       *sceneId,
		Authenticator: *authenticator,
		DrawingId:     *drawingId,
	})
	EnsureNoError(err)
	fmt.Printf("Removed drawing with DrawingId %s from scene with SceneId %s\n", *drawingId, *sceneId)
}

func ClearHandler() {
	// _, err = client.Clear(context.Background(), &ClearRequest{
	// 	SceneId:       registerResult.SceneId,
	// 	Authenticator: registerResult.Authenticator,
	// })
	// EnsureNoError(err)
	// fmt.Printf("Cleared scene with SceneId %s\n", registerResult.SceneId)

}

func DelistHandler() {
	// _, err = client.Delist(context.Background(), &DelistRequest{
	// 	SceneId: registerResult.SceneId,
	// 	Authenticator: registerResult.Authenticator,
	// })
	// EnsureNoError(err)

	// fmt.Printf("Delisted scene with SceneId: %s\n", registerResult.SceneId)
}

var Actions = map[string]func(){
	"Register": RegisterHandler,
	"Draw":     DrawHandler,
	"Poll":     PollHandler,
	"Remove":   RemoveHandler,
	"Clear":    ClearHandler,
	"Delist":   DelistHandler,
}

func main() {
	flag.Parse()

	var conn *grpc.ClientConn

	client, conn = NewClient()
	defer conn.Close()

	fn, ok := Actions[*action]

	if !ok {
		log.Fatalf("Unknown action: %s\n", *action)
	}

	fn()
}
