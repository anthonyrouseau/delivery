package main

import (
	"context"
	"log"
	"time"

	as "github.com/anthonyrouseau/delivery/account_service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	address = "localhost:3000"
)

func main() {
	log.Println("starting client")
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	log.Println("connected to server")
	defer conn.Close()
	c := as.NewAccountServiceClient(conn)
	log.Println("new account client made")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*4)
	defer cancel()
	log.Println("context created")
	createReq := &as.AccountRequest{Username: "Anthony", Password: "kdbakjfdnlkafnd"}
	r, err := c.CreateAccount(ctx, createReq)
	if err != nil {
		log.Fatalf("could not create account: %v", err)
	}
	log.Printf("Created User: %v", r.User)
	deleteReq := &as.AccountRequest{Username: "Anthony"}
	r, err = c.DeleteAccount(metadata.AppendToOutgoingContext(ctx, "auth-token", r.Token), deleteReq)
	if err != nil {
		log.Fatalf("could not delete account: %v", err)
	}
	log.Printf("Deleted User: %v", r.User)
}
