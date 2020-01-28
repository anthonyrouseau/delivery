package main

import (
	"context"
	"log"
	"time"

	ps "github.com/anthonyrouseau/delivery/place_service"
	"google.golang.org/grpc"
)

const (
	address = "localhost:3002"
)

func main() {
	log.Println("starting client")
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	log.Println("connected to server")
	defer conn.Close()
	c := ps.NewPlaceServiceClient(conn)
	log.Println("new account client made")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
	defer cancel()
	log.Println("building request")
	req := &ps.PlaceRequest{
		Coordinates: &ps.Coordinates{
			Lat: 34.083739,
			Lng: -118.3323519,
		},
		Types: []ps.PlaceType{ps.PlaceType_PLACE_TYPE_BEAUTY_STORE},
	}
	log.Println("getting nearby places")
	res, err := c.GetNearbyPlaces(ctx, req)
	if err != nil {
		log.Fatalf("Failed GetNearbyPlaces: %v", err)
	}
	log.Println(res.Message)
	for i, p := range res.Places {
		log.Printf("Place %d: %+v", i+1, p)
	}
}
