package main

import (
	"context"
	"log"
	"math"
	"net"

	ps "github.com/anthonyrouseau/delivery/place_service"
	"google.golang.org/grpc"
	"googlemaps.github.io/maps"
)

const (
	port   = ":3002"
	gcpKey = ""
)

type server struct {
	ps.UnimplementedPlaceServiceServer
}

func (s *server) GetNearbyPlaces(ctx context.Context, pr *ps.PlaceRequest) (*ps.PlaceResponse, error) {
	storeNames := getStoreNames(pr.Types)
	placeRequests := buildRequests(storeNames, pr)
	c, err := maps.NewClient(maps.WithAPIKey(gcpKey))
	if err != nil {
		log.Printf("Failed making maps client")
		return nil, err
	}
	origins := []string{}
	dest := maps.LatLng{Lat: pr.Coordinates.Lat, Lng: pr.Coordinates.Lng}
	destinations := []string{dest.String()}
	placeResponses := []*maps.FindPlaceFromTextResponse{}
	for i, r := range placeRequests {
		res, err := sendPlaceRequest(c, r)
		if err != nil {
			log.Printf("Error occured for response %d: %v", i+1, err)
		}
		placeResponses = append(placeResponses, res)
		origins = append(origins, res.Candidates[0].Geometry.Location.String())
	}
	distanceDurations, err := getDistanceDurations(c, destinations, origins)
	if err != nil {
		log.Printf("Error occured for distances: %v", err)
	}
	log.Printf("Distances: %+v", distanceDurations)
	places := []*ps.Place{}
	for _, p := range placeResponses {
		data := p.Candidates[0]
		ddKey := data.Geometry.Location.String() // key used to match places to distanceDuration data
		distance := distanceDurations[ddKey].Distance
		duration := distanceDurations[ddKey].Duration
		places = append(places, &ps.Place{
			Name:    data.Name,
			Address: data.FormattedAddress,
			Coordinates: &ps.Coordinates{
				Lat: data.Geometry.Location.Lat,
				Lng: data.Geometry.Location.Lng,
			},
			Type: ps.PlaceType_PLACE_TYPE_UNSPECIFIED, // TODO: need to pass type information when getting store names or drop field
			Distance: &ps.Distance{
				Text:  distance.HumanReadable,
				Value: int32(distance.Meters),
			},
			Duration: &ps.Duration{
				Text:  duration.String(),
				Value: int32(math.Ceil(duration.Seconds())),
			},
		})
	}
	return &ps.PlaceResponse{Places: places, Message: "Found Nearby Places"}, nil
}

func getStoreNames(types []ps.PlaceType) []string {
	names := []string{}
	beauty := []string{"Ulta Beauty", "Sephora"}
	clothing := []string{"Zara", "GUESS", "Urban Outfitters"}
	stores := [][]string{
		beauty,
		clothing,
	}
	for _, t := range types {
		switch t {
		case ps.PlaceType_PLACE_TYPE_UNSPECIFIED:
			for _, s := range stores {
				names = append(names, s...)
			}
		case ps.PlaceType_PLACE_TYPE_BEAUTY_STORE:
			names = append(names, beauty...)
		case ps.PlaceType_PLACE_TYPE_CLOTHING_STORE:
			names = append(names, clothing...)
		}
	}
	return names
}

func buildRequests(names []string, pr *ps.PlaceRequest) []*maps.FindPlaceFromTextRequest {
	requests := []*maps.FindPlaceFromTextRequest{}
	for _, n := range names {
		findPlaceRequest := &maps.FindPlaceFromTextRequest{
			LocationBias: maps.FindPlaceFromTextLocationBiasCircular,
			LocationBiasCenter: &maps.LatLng{
				Lat: pr.Coordinates.Lat,
				Lng: pr.Coordinates.Lng,
			},
			LocationBiasRadius: 5000,
			Fields: []maps.PlaceSearchFieldMask{
				maps.PlaceSearchFieldMaskFormattedAddress,
				maps.PlaceSearchFieldMaskGeometryLocation,
				maps.PlaceSearchFieldMaskName,
			},
			Input:     n,
			InputType: maps.FindPlaceFromTextInputTypeTextQuery,
		}
		requests = append(requests, findPlaceRequest)
	}
	return requests
}

func sendPlaceRequest(c *maps.Client, req *maps.FindPlaceFromTextRequest) (*maps.FindPlaceFromTextResponse, error) {
	res, err := c.FindPlaceFromText(context.Background(), req)
	if err != nil {
		return &res, err
	}
	return &res, nil
}

func getDistanceDurations(c *maps.Client, dest, origins []string) (map[string]*maps.DistanceMatrixElement, error) {
	req := &maps.DistanceMatrixRequest{
		Origins:      origins,
		Destinations: dest,
		Mode:         maps.TravelModeDriving,
		Units:        maps.UnitsImperial,
	}
	res, err := c.DistanceMatrix(context.Background(), req)
	if err != nil {
		return nil, err
	}
	distances := make(map[string]*maps.DistanceMatrixElement)
	for i, r := range origins {
		distances[r] = res.Rows[i].Elements[0]
	}
	return distances, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	ps.RegisterPlaceServiceServer(s, &server{})
	log.Println("place server running...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
