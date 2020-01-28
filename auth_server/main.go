package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	auth "github.com/anthonyrouseau/delivery/auth_service"
	jwt "github.com/dgrijalva/jwt-go"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
)

const (
	port       = ":3001"
	authSecret = "auth-secret"
)

type server struct {
	auth.UnimplementedAuthServiceServer
	db *sql.DB
}

func (s *server) HashPassword(ctx context.Context, hr *auth.HashRequest) (*auth.HashReply, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(hr.Password), 14)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &auth.HashReply{PasswordHash: string(bytes)}, nil
}

func (s *server) Authenticate(ctx context.Context, ar *auth.AuthenticationRequest) (*auth.AuthenticationReply, error) {
	token, err := jwt.Parse(ar.Token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(authSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("Invalid Auth Token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("Failed to map token claims")
	}
	exp, err := strconv.ParseInt(claims["exp"].(string), 10, 64)
	if err != nil {
		return nil, errors.New("Failed to convert exp claim to int64")
	}
	if exp < time.Now().Unix() {
		return nil, errors.New("Expired Auth Token")
	}
	username, ok := claims["username"].(string)
	if !ok {
		return nil, errors.New("Failed to convert username claim to string")
	}
	log.Printf("exp claim: %d, username claim: %s", exp, username)
	return &auth.AuthenticationReply{Authenticated: true, Claims: &auth.TokenClaims{Username: username, Exp: exp}}, nil
}

func (s *server) GetAuthToken(ctx context.Context, ar *auth.AuthTokenRequest) (*auth.AuthTokenReply, error) {
	var hashed string
	err := s.db.QueryRow("SELECT Password FROM accounts WHERE Username=?", ar.Username).Scan(&hashed)
	if err != nil {
		log.Printf("Error finding account: %v", err)
		return nil, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(hashed), []byte(ar.Password))
	if err != nil {
		log.Println("Error with bycrypt compare")
		return &auth.AuthTokenReply{Message: "Failed to get Auth Token", Token: ""}, err
	}
	expAt := int64(60*60*24*7) + time.Now().Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": ar.Username,
		"exp":      strconv.FormatInt(expAt, 10),
	})
	log.Println("Created token... preparing to sign")
	signedString, err := token.SignedString([]byte(authSecret))
	if err != nil {
		log.Println("Error signing token")
		return &auth.AuthTokenReply{Message: "Failed to get Auth Token", Token: ""}, err
	}
	log.Printf("Created new token: %s", signedString)
	return &auth.AuthTokenReply{Message: "Auth Token Success", Token: signedString}, nil
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
	s := grpc.NewServer()
	auth.RegisterAuthServiceServer(s, &server{db: db})
	log.Println("auth server running...")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
