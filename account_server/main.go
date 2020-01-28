package main

import (
	"context"
	"errors"
	"log"
	"net"

	as "github.com/anthonyrouseau/delivery/account_service"
	auth "github.com/anthonyrouseau/delivery/auth_service"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	port = ":3000"
)

type accountServer struct {
	as.UnimplementedAccountServiceServer
	db *sql.DB
}

func (s *accountServer) CreateAccount(ctx context.Context, ar *as.AccountRequest) (*as.AccountReply, error) {
	address := "localhost:3001"
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	authClient := auth.NewAuthServiceClient(conn)
	hashReq := &auth.HashRequest{Password: ar.Password}
	hashRes, err := authClient.HashPassword(ctx, hashReq)
	if err != nil {
		return &as.AccountReply{Message: "Unable to create account", User: nil}, err
	}
	res, err := s.db.Exec("INSERT INTO accounts (Username, Password) VALUES (?, ?)", ar.Username, hashRes.PasswordHash)
	if err != nil {
		log.Println(err)
		return &as.AccountReply{Message: "Unable to create account", User: nil}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		log.Println(err)
	}
	newUser := &as.User{
		Username: ar.Username,
		Id:       id,
	}
	tokenRes, err := authClient.GetAuthToken(ctx, &auth.AuthTokenRequest{Username: ar.Username, Password: ar.Password})
	if err != nil {
		log.Println("Error getting token")
		log.Println(err)
	}
	return &as.AccountReply{Message: "Account created", User: newUser, Token: tokenRes.Token}, nil
}

func (s *accountServer) DeleteAccount(ctx context.Context, ar *as.AccountRequest) (*as.AccountReply, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("Failed getting context metadata")
	}
	verifiedUsername, ok := md["username"]
	if !ok {
		return nil, errors.New("Failed to get verified username")
	}
	if verifiedUsername[0] != ar.Username {
		return nil, errors.New("Delete Failed. Request did not match verified name")
	}
	res, err := s.db.Exec("DELETE FROM accounts WHERE Username=?", ar.Username)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		log.Println(err)
	}
	deletedUser := &as.User{
		Username: ar.Username,
		Id:       0,
	}
	if affected == 0 {
		return nil, errors.New("unable to delete account")
	}
	return &as.AccountReply{Message: "account deleted", User: deletedUser}, nil
}

func main() {
	db, err := sql.Open("mysql", "test:dbtestpass1!@tcp(127.0.0.1:3306)/test-1")
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer(grpc.UnaryInterceptor(authInterceptor))
	as.RegisterAccountServiceServer(s, &accountServer{db: db})
	log.Println("account server running...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if info.FullMethod == "/account_service.Account/CreateAccount" {
		h, err := handler(ctx, req)
		return h, err
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("Failed getting context metadata")
	}
	token, ok := md["auth-token"]
	if !ok {
		return nil, errors.New("Missing Auth Token in context metadata")
	}
	address := "localhost:3001"
	conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	authClient := auth.NewAuthServiceClient(conn)
	authReq := &auth.AuthenticationRequest{Token: token[0]} //md returns slice of values so we pass first value
	authRes, err := authClient.Authenticate(ctx, authReq)
	if err != nil {
		log.Printf("Error with authentication: %v", err)
		return nil, err
	}
	if authRes.Authenticated != true {
		log.Println("Authenticated value was not true")
		return nil, errors.New("Authentication failed")
	}
	newContext := metadata.NewIncomingContext(ctx, md)
	newMd, ok := metadata.FromIncomingContext(newContext)
	if !ok {
		return nil, errors.New("Failed getting new context metadata")
	}
	newMd.Set("username", authRes.Claims.Username)
	h, err := handler(newContext, req)
	return h, err

}
